package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/scoring"
	"github.com/dsionov/carwatch/internal/storage"
)

const (
	fetchTimeout = 60 * time.Second
	// kmEnrichTimeout bounds per-item mileage/city fetches after the list crawl.
	kmEnrichTimeout       = 25 * time.Minute
	maxBackoff            = 4.0
	minBackoff            = 1.0
	pruneInterval         = 24 * time.Hour
	maxRetries            = 3
	retryBaseDelay        = 2 * time.Second
	defaultConcurrency    = 4
	notificationPruneAge      = 48 * time.Hour
	priceHistoryRetention     = 90 * 24 * time.Hour
	listingHistoryRetention   = 90 * 24 * time.Hour
)

type CatalogIngester interface {
	Ingest(ctx context.Context, e catalog.IngestEntry)
	Flush(ctx context.Context)
}

// CarNameResolver translates numeric manufacturer/model IDs into
// human-readable names for log messages.
type CarNameResolver interface {
	ManufacturerName(id int) string
	ModelName(manufacturerID, modelID int) string
}

// KmEnricher fills in missing km data by fetching individual listing pages.
type KmEnricher interface {
	Enrich(ctx context.Context, listings []model.RawListing) int
}

type Scheduler struct {
	cfgMu             sync.RWMutex
	cfg               *config.Config
	configPath        string
	fetcher           fetcher.Fetcher
	stores            Stores
	notifier          notifier.Notifier
	logger            *slog.Logger
	loc               *time.Location
	boMu              sync.RWMutex
	backoffMultiplier float64
	lastPruneTime     time.Time
	observer          CycleObserver
	fetcherFactory    *fetcher.Factory
	catalogIngester   CatalogIngester
	carNames          CarNameResolver
	kmEnricher        KmEnricher
	triggerCh         chan struct{}

	langCache   sync.Map
	digestCache sync.Map
	cycleCount  uint64
}

type digestMeta struct {
	mode     string
	interval string
}

// Stores groups all storage interfaces the scheduler depends on.
type Stores struct {
	Dedup        storage.DedupStore
	Queue        storage.NotificationQueue
	Prices       storage.PriceTracker
	Listings     storage.ListingStore
	Searches     storage.SearchStore
	Users        storage.UserStore
	Digests      storage.DigestStore
	Hidden       storage.HiddenListingStore
	Market       storage.MarketStore
	DailyDigests storage.DailyDigestStore
}

type searchResult struct {
	newListings       []model.Listing
	priceDropMessages []string
	listingRecords    []storage.ListingRecord
}

type Options struct {
	Observer         CycleObserver
	Queue            storage.NotificationQueue
	Prices           storage.PriceTracker
	ConfigPath       string
	FetcherFactory   *fetcher.Factory
	ListingStore     storage.ListingStore
	SearchStore      storage.SearchStore
	UserStore        storage.UserStore
	DigestStore      storage.DigestStore
	HiddenStore      storage.HiddenListingStore
	CatalogIngester  CatalogIngester
	CarNames         CarNameResolver
	KmEnricher       KmEnricher
	MarketStore      storage.MarketStore
	DailyDigestStore storage.DailyDigestStore
}

func New(
	cfg *config.Config,
	f fetcher.Fetcher,
	d storage.DedupStore,
	n notifier.Notifier,
	logger *slog.Logger,
	observer CycleObserver,
) (*Scheduler, error) {
	return NewWithOptions(cfg, f, d, n, logger, Options{Observer: observer})
}

func NewWithOptions(
	cfg *config.Config,
	f fetcher.Fetcher,
	d storage.DedupStore,
	n notifier.Notifier,
	logger *slog.Logger,
	opts Options,
) (*Scheduler, error) {
	loc, err := time.LoadLocation(cfg.Polling.Timezone)
	if err != nil {
		return nil, err
	}
	obs := opts.Observer
	if obs == nil {
		obs = nopObserver{}
	}
	return &Scheduler{
		cfg:        cfg,
		configPath: opts.ConfigPath,
		fetcher:    f,
		stores: Stores{
			Dedup:        d,
			Queue:        opts.Queue,
			Prices:       opts.Prices,
			Listings:     opts.ListingStore,
			Searches:     opts.SearchStore,
			Users:        opts.UserStore,
			Digests:      opts.DigestStore,
			Hidden:       opts.HiddenStore,
			Market:       opts.MarketStore,
			DailyDigests: opts.DailyDigestStore,
		},
		notifier:          n,
		logger:            logger,
		loc:               loc,
		backoffMultiplier: 1.0,
		observer:          obs,
		fetcherFactory:    opts.FetcherFactory,
		catalogIngester:   opts.CatalogIngester,
		carNames:          opts.CarNames,
		kmEnricher:        opts.KmEnricher,
		triggerCh:         make(chan struct{}, 1),
	}, nil
}

func (s *Scheduler) TriggerPoll() {
	select {
	case s.triggerCh <- struct{}{}:
	default:
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.cfgMu.RLock()
	logInterval := s.cfg.Polling.Interval
	logJitter := s.cfg.Polling.Jitter
	s.cfgMu.RUnlock()
	s.logger.Info("scheduler started",
		"check_interval", logInterval.String(),
		"jitter", logJitter.String(),
	)

	s.retryPending(ctx)

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)

	cycle := s.runMultiTenantCycle

	if s.isActiveHours() {
		if err := cycle(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			s.logger.Error("initial cycle failed", "error", err)
		}
	}

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		delay := s.nextDelay()

		if !s.isActiveHours() {
			if sleepUntil := s.durationUntilActiveStart(); sleepUntil > 0 {
				s.logger.Info("outside active hours, sleeping until start",
					"sleep", sleepUntil.Round(time.Minute).String(),
				)
				delay = sleepUntil
			}
		}

		s.logger.Info("next scan", "in", delay.Round(time.Second).String())

		timer.Reset(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			s.logger.Info("scheduler stopping")
			return ctx.Err()
		case <-sighup:
			if !timer.Stop() {
				<-timer.C
			}
			s.reloadConfig()
			continue
		case <-s.triggerCh:
			if !timer.Stop() {
				<-timer.C
			}
			s.logger.Info("scan triggered manually")
		case <-timer.C:
		}

		if !s.isActiveHours() {
			continue
		}

		if err := cycle(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			s.logger.Error("scan failed", "error", err)
		}
	}
}

func (s *Scheduler) deliveryFor(ctx context.Context, chatID int64, lang locale.Lang) DeliveryStrategy {
	if s.stores.Digests != nil {
		var mode string
		if v, ok := s.digestCache.Load(chatID); ok {
			if dm, ok := v.(digestMeta); ok {
				mode = dm.mode
			}
		} else {
			m, interval, err := s.stores.Digests.GetDigestMode(ctx, chatID)
			if err != nil {
				if !errors.Is(err, storage.ErrNotFound) {
					s.logger.Error("get digest mode failed", "chat_id", chatID, "error", err)
				}
			} else {
				mode = m
				s.digestCache.Store(chatID, digestMeta{mode: m, interval: interval})
			}
		}
		if mode == "digest" {
			return NewDigestDelivery(s.stores.Digests, lang)
		}
	}
	return NewInstantDelivery(s.notifier, s.stores.Queue, lang, WithLogger(s.logger))
}

func (s *Scheduler) fetcherForSource(source string) fetcher.Fetcher {
	if s.fetcherFactory != nil {
		if f, ok := s.fetcherFactory.Get(source); ok {
			return f
		}
	}
	return s.fetcher
}

func (s *Scheduler) fetchWithRetryUsing(ctx context.Context, f fetcher.Fetcher, params model.SourceParams) ([]model.RawListing, error) {
	var lastErr error
	for attempt := range maxRetries {
		listings, err := f.Fetch(ctx, params)
		if err == nil {
			return listings, nil
		}
		lastErr = err

		if errors.Is(err, fetcher.ErrPartialResults) && len(listings) > 0 {
			s.logger.Warn("partial results (some pages failed)",
				"car", s.carName(params.Manufacturer, params.Model),
				"listings_returned", len(listings),
				"error", err,
			)
			return listings, nil
		}

		if errors.Is(err, fetcher.ErrChallenge) || errors.Is(err, fetcher.ErrCircuitOpen) || errors.Is(err, context.Canceled) {
			return nil, err
		}

		if attempt < maxRetries-1 {
			delay := retryBaseDelay * (1 << attempt)
			s.logger.Warn("fetch failed, retrying",
				"car", s.carName(params.Manufacturer, params.Model),
				"attempt", fmt.Sprintf("%d/%d", attempt+1, maxRetries),
				"retry_in", delay.String(),
				"error", err,
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("all %d fetch attempts failed: %w", maxRetries, lastErr)
}

func (s *Scheduler) nextDelay() time.Duration {
	s.cfgMu.RLock()
	interval := s.cfg.Polling.Interval
	jitterCfg := s.cfg.Polling.Jitter
	s.cfgMu.RUnlock()
	s.boMu.RLock()
	mult := s.backoffMultiplier
	s.boMu.RUnlock()
	base := time.Duration(float64(interval) * mult)
	jitter := jitterCfg
	if jitter > 0 {
		offset := time.Duration(rand.Int64N(int64(2*jitter))) - jitter
		base += offset
	}
	if base < time.Minute {
		base = time.Minute
	}
	return base
}

func (s *Scheduler) isActiveHours() bool {
	s.cfgMu.RLock()
	ah := s.cfg.Polling.ActiveHours
	loc := s.loc
	s.cfgMu.RUnlock()
	if ah == nil {
		return true
	}

	now := time.Now().In(loc)
	currentMinutes := now.Hour()*60 + now.Minute()

	start := parseTimeOfDayOrZero(ah.Start)
	end := parseTimeOfDayOrZero(ah.End)

	if start == 0 && end == 0 {
		return true
	}

	if start < end {
		return currentMinutes >= start && currentMinutes < end
	}
	// Overnight window (e.g. 22:00-06:00).
	return currentMinutes >= start || currentMinutes < end
}

func (s *Scheduler) durationUntilActiveStart() time.Duration {
	s.cfgMu.RLock()
	ah := s.cfg.Polling.ActiveHours
	loc := s.loc
	s.cfgMu.RUnlock()
	if ah == nil {
		return 0
	}

	startMinutes := parseTimeOfDayOrZero(ah.Start)
	h, m := startMinutes/60, startMinutes%60
	now := time.Now().In(loc)
	target := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	if !target.After(now) {
		nextDay := now.AddDate(0, 0, 1)
		target = time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), h, m, 0, 0, loc)
	}
	return target.Sub(now)
}

func parseTimeOfDayOrZero(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}

func (s *Scheduler) reloadConfig() {
	if s.configPath == "" {
		s.logger.Warn("SIGHUP received but no config path set, ignoring")
		return
	}
	s.logger.Info("SIGHUP received, reloading config", "path", s.configPath)
	newCfg, err := config.Load(s.configPath)
	if err != nil {
		s.logger.Error("config reload failed, keeping current config", "error", err)
		return
	}
	loc, err := time.LoadLocation(newCfg.Polling.Timezone)
	if err != nil {
		s.logger.Error("config reload: invalid timezone, keeping current", "timezone", newCfg.Polling.Timezone, "error", err)
		return
	}
	s.cfgMu.Lock()
	s.cfg = newCfg
	s.loc = loc
	s.cfgMu.Unlock()
	s.logger.Info("config reloaded")
}

func (s *Scheduler) retryPending(ctx context.Context) {
	if s.stores.Queue == nil {
		return
	}
	pending, err := s.stores.Queue.PendingNotifications(ctx)
	if err != nil {
		s.logger.Error("failed to load pending notifications", "error", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	s.logger.Info("retrying queued messages", "count", len(pending))
	for _, p := range pending {
		if notifier.IsMalformedMessage(p.Payload) {
			s.logger.Error("purging malformed pending notification",
				"id", p.ID,
				"recipient", maskPhone(p.Recipient),
				"msg_len", len(p.Payload),
				"msg_preview", truncateStr(p.Payload, 200),
			)
			if err := s.stores.Queue.AckNotification(ctx, p.ID); err != nil {
				s.logger.Error("ack malformed notification failed", "id", p.ID, "error", err)
			}
			continue
		}
		s.logger.Debug("retrying pending notification",
			"id", p.ID,
			"recipient", maskPhone(p.Recipient),
			"msg_len", len(p.Payload),
			"msg_preview", truncateStr(p.Payload, 100),
		)
		if err := s.notifier.NotifyRaw(ctx, p.Recipient, p.Payload); err != nil {
			if errors.Is(err, notifier.ErrRecipientBlocked) {
				s.logger.Warn("purging notification for unreachable recipient",
					"id", p.ID,
					"recipient", maskPhone(p.Recipient),
					"error", err,
				)
				if ackErr := s.stores.Queue.AckNotification(ctx, p.ID); ackErr != nil {
					s.logger.Error("ack unreachable notification failed", "id", p.ID, "error", ackErr)
				}
				continue
			}
			s.logger.Error("retry notification failed",
				"id", p.ID,
				"recipient", maskPhone(p.Recipient),
				"error", err,
			)
			continue
		}
		if err := s.stores.Queue.AckNotification(ctx, p.ID); err != nil {
			s.logger.Error("ack notification failed", "id", p.ID, "error", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Scheduler) runMultiTenantCycle(ctx context.Context) error {
	s.cycleCount++
	cycle := s.cycleCount
	cycleStart := time.Now()

	s.logger.Info("checking for new listings", "scan", cycle)

	s.langCache.Range(func(k, _ any) bool { s.langCache.Delete(k); return true })
	s.digestCache.Range(func(k, _ any) bool { s.digestCache.Delete(k); return true })

	searches, err := s.stores.Searches.ListAllActiveSearches(ctx)
	if err != nil {
		return fmt.Errorf("load searches: %w", err)
	}

	s.pruneOldData(ctx)
	s.processExpiredPremium(ctx)

	if len(searches) == 0 {
		s.logger.Info("scan complete (no active searches)", "scan", cycle, "elapsed", time.Since(cycleStart).Round(time.Millisecond))
		return nil
	}

	marketCache := s.buildMarketCache(ctx)
	groups := GroupSearches(searches)
	s.logger.Info("active searches loaded",
		"scan", cycle,
		"car_models", len(groups),
		"searches", len(searches),
	)

	allFailed, stats := s.runFetchGroups(ctx, groups, marketCache)

	if s.catalogIngester != nil {
		s.catalogIngester.Flush(ctx)
	}

	s.processDigests(ctx)
	s.processDailyDigests(ctx)

	s.logger.Info("scan complete",
		"scan", cycle,
		"elapsed", time.Since(cycleStart).Round(time.Millisecond),
		"car_models", len(groups),
		"searches", len(searches),
		"listings_checked", stats.listingsFetched,
		"new_matches", stats.newListings,
		"notifications_sent", stats.notificationsSent,
		"failed", stats.groupsFailed,
	)

	if allFailed && len(groups) > 0 {
		s.observer.RecordError()
		return fmt.Errorf("all %d car models failed to fetch", len(groups))
	}

	s.observer.RecordSuccess()
	return nil
}

func (s *Scheduler) pruneOldData(ctx context.Context) {
	if time.Since(s.lastPruneTime) <= pruneInterval {
		return
	}
	s.cfgMu.RLock()
	pruneAfter := s.cfg.Storage.PruneAfter
	s.cfgMu.RUnlock()
	if pruneAfter > 0 {
		pruned, err := s.stores.Dedup.Prune(ctx, pruneAfter)
		if err != nil {
			s.logger.Error("prune failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listings", "count", pruned)
		}
	}
	if s.stores.Queue != nil {
		pruned, err := s.stores.Queue.PruneNotifications(ctx, notificationPruneAge)
		if err != nil {
			s.logger.Error("prune notifications failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned expired notifications", "count", pruned)
		}
	}
	if s.stores.Prices != nil {
		pruned, err := s.stores.Prices.PrunePrices(ctx, priceHistoryRetention)
		if err != nil {
			s.logger.Error("prune prices failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old price history", "count", pruned)
		}
	}
	if s.stores.Listings != nil {
		pruned, err := s.stores.Listings.PruneListings(ctx, listingHistoryRetention)
		if err != nil {
			s.logger.Error("prune listing history failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listing history", "count", pruned)
		}
	}
	s.lastPruneTime = time.Now()
}

func (s *Scheduler) buildMarketCache(ctx context.Context) *scoring.MarketCache {
	if s.stores.Market == nil {
		return nil
	}
	listings, err := s.stores.Market.MarketListings(ctx)
	if err != nil {
		s.logger.Error("load market data failed", "error", err)
		return nil
	}
	data := make([]scoring.ListingData, len(listings))
	for i, l := range listings {
		data[i] = scoring.ListingData{
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
		}
	}
	return scoring.NewMarketCache(data)
}

type cycleStats struct {
	listingsFetched   int
	newListings       int
	notificationsSent int
	groupsFailed      int
}

// runFetchGroups runs processGroup for each canonical group with bounded concurrency.
// It reports whether every group failed and aggregate stats.
func (s *Scheduler) runFetchGroups(ctx context.Context, groups []CanonicalGroup, marketCache *scoring.MarketCache) (bool, cycleStats) {
	s.cfgMu.RLock()
	concurrency := s.cfg.Polling.MaxConcurrentFetches
	s.cfgMu.RUnlock()
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allFailed := true
	var stats cycleStats

	cancelled := false
	for _, group := range groups {
		if cancelled {
			break
		}
		select {
		case <-ctx.Done():
			cancelled = true
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(g CanonicalGroup) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("unexpected crash while fetching listings",
						"car", s.carName(g.Manufacturer, g.Model),
						"panic", r,
						"stack", string(debug.Stack()))
					s.observer.RecordError()
				}
			}()

			gs, err := s.processGroup(ctx, g, marketCache)
			if err != nil {
				s.logger.Error("failed to fetch listings",
					"car", s.carName(g.Manufacturer, g.Model),
					"source", g.Source,
					"error", err)
				if errors.Is(err, fetcher.ErrChallenge) {
					s.boMu.Lock()
					s.backoffMultiplier = min(s.backoffMultiplier*2, maxBackoff)
					s.boMu.Unlock()
				}
				mu.Lock()
				stats.groupsFailed++
				mu.Unlock()
				return
			}
			mu.Lock()
			allFailed = false
			stats.listingsFetched += gs.listingsFetched
			stats.newListings += gs.newListings
			stats.notificationsSent += gs.notificationsSent
			mu.Unlock()
			s.boMu.Lock()
			s.backoffMultiplier = max(s.backoffMultiplier/2, minBackoff)
			s.boMu.Unlock()
		}(group)
	}
	wg.Wait()
	return allFailed, stats
}

func (s *Scheduler) processGroup(ctx context.Context, group CanonicalGroup, marketCache *scoring.MarketCache) (cycleStats, error) {
	groupStart := time.Now()
	s.logger.Debug("fetching car model",
		"car", s.carName(group.Manufacturer, group.Model),
		"searches", len(group.Searches),
	)
	raw, source, err := s.fetchAndEnrich(ctx, group)
	if err != nil {
		return cycleStats{}, err
	}

	var gs cycleStats
	gs.listingsFetched = len(raw)

	for _, search := range group.Searches {
		filtered := filter.Apply(buildFilterCriteria(search), raw)
		s.logger.Debug("search filter applied",
			"search_id", search.ID,
			"chat_id", search.ChatID,
			"total_raw", len(raw),
			"after_filter", len(filtered),
		)
		lang := s.userLang(ctx, search.ChatID)
		sr := s.processSearchListings(ctx, search, filtered, marketCache, lang)
		persistOK := true
		if s.stores.Listings != nil && len(sr.listingRecords) > 0 {
			if err := s.persistListings(ctx, sr.listingRecords); err != nil {
				persistOK = false
			}
		}
		if !persistOK {
			// Persist failed: still deliver price-drop messages (already saved
			// individually in tryPriceDropListing), but drop new-listing
			// notifications since they weren't persisted.
			sr.newListings = nil
			sr.listingRecords = nil
		}
		gs.newListings += len(sr.newListings)
		delivered := s.deliverResults(ctx, search, lang, sr)
		if delivered {
			gs.notificationsSent++
		}
	}

	s.logger.Info("car model checked",
		"car", s.carName(group.Manufacturer, group.Model),
		"source", source,
		"listings", len(raw),
		"new_matches", gs.newListings,
		"searches", len(group.Searches),
		"elapsed", time.Since(groupStart).Round(time.Millisecond),
	)

	return gs, nil
}

func (s *Scheduler) fetchAndEnrich(ctx context.Context, group CanonicalGroup) ([]model.RawListing, string, error) {
	listCtx, cancelList := context.WithTimeout(ctx, fetchTimeout)
	defer cancelList()

	source := group.Source
	if source == "" {
		source = "yad2"
	}
	activeFetcher := s.fetcherForSource(source)
	fetchStart := time.Now()
	raw, err := s.fetchWithRetryUsing(listCtx, activeFetcher, group.Params)
	s.observer.RecordFetch(source, time.Since(fetchStart), err)
	if err != nil {
		return nil, source, err
	}

	if s.catalogIngester != nil {
		for _, l := range raw {
			s.catalogIngester.Ingest(ctx, catalog.IngestEntry{
				ManufacturerID:     l.ManufacturerID,
				ManufacturerName:   l.Manufacturer,
				ManufacturerNameHe: l.ManufacturerNameHe,
				ModelID:            l.ModelID,
				ModelName:          l.Model,
				ModelNameHe:        l.ModelNameHe,
			})
		}
	}

	if source == "yad2" && s.kmEnricher != nil {
		enrichCtx, cancelEnrich := context.WithTimeout(ctx, kmEnrichTimeout)
		defer cancelEnrich()
		enriched := s.kmEnricher.Enrich(enrichCtx, raw)
		if enriched > 0 {
			s.logger.Info("mileage data enriched",
				"car", s.carName(group.Manufacturer, group.Model),
				"enriched", enriched,
				"total", len(raw),
			)
			if s.stores.Listings != nil {
				s.backfillEnrichedListings(ctx, raw)
			}
		}

		// Pre-fill: load previously-known km/city/image from DB for listings
		// that still lack km after enrichment.
		if s.stores.Listings != nil {
			s.prefillFromDB(ctx, raw)
		}
	}

	return raw, source, nil
}

// prefillFromDB fills in km/city/image from listing_history for listings
// that the enricher could not reach this cycle. Once a listing's km is
// learned in any previous cycle, it is remembered here.
func (s *Scheduler) prefillFromDB(ctx context.Context, listings []model.RawListing) {
	var tokens []string
	for i := range listings {
		if listings[i].Km <= 0 || listings[i].City == "" || listings[i].ImageURL == "" {
			tokens = append(tokens, listings[i].Token)
		}
	}
	if len(tokens) == 0 {
		return
	}

	data, err := s.stores.Listings.LookupEnrichmentData(ctx, tokens)
	if err != nil {
		s.logger.Error("prefill from DB failed", "error", err)
		return
	}
	if len(data) == 0 {
		return
	}

	filled := 0
	for i := range listings {
		rec, ok := data[listings[i].Token]
		if !ok {
			continue
		}
		changed := false
		if listings[i].Km <= 0 && rec.Km > 0 {
			listings[i].Km = rec.Km
			changed = true
		}
		if listings[i].City == "" && rec.City != "" {
			listings[i].City = rec.City
			changed = true
		}
		if listings[i].ImageURL == "" && rec.ImageURL != "" {
			listings[i].ImageURL = rec.ImageURL
			changed = true
		}
		if changed {
			filled++
		}
	}
	if filled > 0 {
		s.logger.Info("prefilled from DB", "filled", filled, "looked_up", len(tokens))
	}
}

// backfillEnrichedListings upserts listing_history for listings that gained
// km/city/image data during enrichment, ensuring the DB is updated even for
// previously-seen tokens.
func (s *Scheduler) backfillEnrichedListings(ctx context.Context, listings []model.RawListing) {
	var toUpdate []storage.ListingRecord
	for _, l := range listings {
		if l.Km <= 0 {
			continue
		}
		toUpdate = append(toUpdate, storage.ListingRecord{
			Token:        l.Token,
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			City:         l.City,
			PageLink:     l.PageLink,
			ImageURL:     l.ImageURL,
		})
	}
	if len(toUpdate) == 0 {
		return
	}
	if err := s.stores.Listings.BackfillListings(ctx, toUpdate); err != nil {
		s.logger.Error("km backfill failed", "count", len(toUpdate), "error", err)
	}
}

func buildFilterCriteria(search storage.Search) model.FilterCriteria {
	criteria := model.FilterCriteria{
		ModelID:     search.Model,
		YearMin:     search.YearMin,
		YearMax:     search.YearMax,
		PriceMax:    search.PriceMax,
		EngineMinCC: float64(search.EngineMinCC),
		MaxKm:       search.MaxKm,
		MaxHand:     search.MaxHand,
	}

	if search.Keywords != "" {
		for _, kw := range strings.Split(search.Keywords, ",") {
			if kw = strings.TrimSpace(kw); kw != "" {
				criteria.Keywords = append(criteria.Keywords, kw)
			}
		}
	}
	if search.ExcludeKeys != "" {
		for _, kw := range strings.Split(search.ExcludeKeys, ",") {
			if kw = strings.TrimSpace(kw); kw != "" {
				criteria.ExcludeKeys = append(criteria.ExcludeKeys, kw)
			}
		}
	}

	return criteria
}

func filterHiddenListings(filtered []model.RawListing, hidden map[string]bool) []model.RawListing {
	if len(hidden) == 0 {
		return filtered
	}
	out := make([]model.RawListing, 0, len(filtered))
	for _, l := range filtered {
		if !hidden[l.Token] {
			out = append(out, l)
		}
	}
	return out
}

func (s *Scheduler) loadHiddenTokens(ctx context.Context, chatID int64) map[string]bool {
	if s.stores.Hidden == nil {
		return nil
	}
	tokens, err := s.stores.Hidden.ListHiddenTokens(ctx, chatID)
	if err != nil {
		s.logger.Error("load hidden tokens failed", "chat_id", chatID, "error", err)
		return nil
	}
	return tokens
}

func (s *Scheduler) deduplicateListings(ctx context.Context, token string, chatID, searchID int64) (isNew bool, ok bool) {
	isNew, err := s.stores.Dedup.ClaimNew(ctx, token, chatID, searchID)
	if err != nil {
		s.logger.Error("claim failed", "token", token, "error", err)
		return false, false
	}
	return isNew, true
}

func (s *Scheduler) tryPriceDropListing(ctx context.Context, search storage.Search, l model.RawListing, lang locale.Lang, marketCache *scoring.MarketCache, out *searchResult) bool {
	if s.stores.Prices == nil || l.Price <= 0 {
		return false
	}
	oldPrice, changed, err := s.stores.Prices.RecordPrice(ctx, l.Token, l.Price)
	if err != nil {
		s.logger.Error("record price failed", "token", l.Token, "error", err)
		return false
	}
	if !changed || l.Price >= oldPrice {
		return false
	}
	s.logger.Info("price drop detected",
		"token", l.Token,
		"old_price", oldPrice,
		"new_price", l.Price,
	)
	listing := model.Listing{RawListing: l, SearchName: search.Name}
	fp := scoring.FitnessParams{
		Price: l.Price, Km: l.Km, Hand: l.Hand, Year: l.Year,
		EngineVolume: l.EngineVolume, PriceMax: search.PriceMax,
		MaxKm: search.MaxKm, MaxHand: search.MaxHand,
		YearMin: search.YearMin, YearMax: search.YearMax,
		EngineMinCC: search.EngineMinCC,
	}
	if marketCache != nil {
		if median, _, _, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
			fp.MedianPrice = median
		}
	}
	listing.FitnessScore = scoring.FitnessScore(fp)
	out.priceDropMessages = append(out.priceDropMessages, notifier.FormatPriceDrop(listing, oldPrice, lang))
	if s.stores.Listings != nil {
		rec := storage.ListingRecord{
			Token: l.Token, ChatID: search.ChatID, SearchID: search.ID, SearchName: search.Name,
			Manufacturer: l.Manufacturer, Model: l.Model, SubModel: l.SubModel,
			Year: l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
			City: l.City, PageLink: l.PageLink, ImageURL: l.ImageURL,
			EngineVolume: l.EngineVolume, HorsePower: l.HorsePower,
			EngineType: l.EngineType, GearBox: l.GearBox, Description: l.Description,
			IsCommercial: l.Commercial,
			FitnessScore: &listing.FitnessScore, FirstSeenAt: time.Now(),
		}
		if marketCache != nil {
			if median, medKm, cohort, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
				ds := scoring.ScoreWithKm(l.Price, l.Km, median, medKm)
				rec.MedianPrice = &median
				rec.CohortSize = &cohort
				rec.DealScore = &ds
			}
		}
		if err := s.stores.Listings.SaveListing(ctx, rec); err != nil {
			s.logger.Error("save price-drop listing failed",
				"token", l.Token,
				"chat_id", search.ChatID,
				"error", err,
			)
		}
	}
	return true
}

func (s *Scheduler) scoreAndRecordListings(search storage.Search, l model.RawListing, marketCache *scoring.MarketCache) model.Listing {
	listing := model.Listing{RawListing: l, SearchName: search.Name}

	// Look up market median before fitness scoring so the price dimension
	// can score against market value instead of just the budget cap.
	var medianPrice, medianKm, cohort int
	var marketOK bool
	if marketCache != nil && l.Price > 0 {
		medianPrice, medianKm, cohort, marketOK = marketCache.Lookup(l.Manufacturer, l.Model, l.Year)
	}

	fp := scoring.FitnessParams{
		Price:        l.Price,
		Km:           l.Km,
		Hand:         l.Hand,
		Year:         l.Year,
		EngineVolume: l.EngineVolume,
		PriceMax:     search.PriceMax,
		MaxKm:        search.MaxKm,
		MaxHand:      search.MaxHand,
		YearMin:      search.YearMin,
		YearMax:      search.YearMax,
		EngineMinCC:  search.EngineMinCC,
	}
	if marketOK {
		fp.MedianPrice = medianPrice
	}

	detailed := scoring.FitnessScoreDetailed(fp)
	listing.FitnessScore = detailed.Total
	listing.FitnessBreakdown = make([]model.FitnessDim, len(detailed.Dims))
	for i, d := range detailed.Dims {
		listing.FitnessBreakdown[i] = model.FitnessDim{
			Name: d.Name, Score: d.Score, Weight: d.Weight,
		}
	}
	if marketOK {
		listing.DealScore = &model.ScoreInfo{
			Score:       scoring.ScoreWithKm(l.Price, l.Km, medianPrice, medianKm),
			MedianPrice: medianPrice,
			MedianKm:    medianKm,
			CohortSize:  cohort,
		}
	}
	return listing
}

func buildNotifications(search storage.Search, listing model.Listing, out *searchResult) {
	out.newListings = append(out.newListings, listing)
	rec := storage.ListingRecord{
		Token: listing.Token, ChatID: search.ChatID, SearchID: search.ID, SearchName: search.Name,
		Manufacturer: listing.Manufacturer, Model: listing.Model, SubModel: listing.SubModel,
		Year: listing.Year, Price: listing.Price, Km: listing.Km, Hand: listing.Hand,
		City: listing.City, PageLink: listing.PageLink, ImageURL: listing.ImageURL,
		EngineVolume: listing.EngineVolume, HorsePower: listing.HorsePower,
		EngineType: listing.EngineType, GearBox: listing.GearBox, Description: listing.Description,
		IsCommercial: listing.Commercial,
		FitnessScore: &listing.FitnessScore, FirstSeenAt: time.Now(),
	}
	if listing.DealScore != nil {
		rec.MedianPrice = &listing.DealScore.MedianPrice
		rec.CohortSize = &listing.DealScore.CohortSize
		rec.DealScore = &listing.DealScore.Score
	}
	out.listingRecords = append(out.listingRecords, rec)
}

func (s *Scheduler) processSearchListings(ctx context.Context, search storage.Search, filtered []model.RawListing, marketCache *scoring.MarketCache, lang locale.Lang) searchResult {
	var out searchResult
	hidden := s.loadHiddenTokens(ctx, search.ChatID)
	for _, l := range filterHiddenListings(filtered, hidden) {
		if !storage.RawListingMatchesSellerFilter(l.Commercial, search.SellerFilter) {
			continue
		}
		isNew, ok := s.deduplicateListings(ctx, l.Token, search.ChatID, search.ID)
		if !ok {
			continue
		}
		if s.tryPriceDropListing(ctx, search, l, lang, marketCache, &out) {
			continue
		}
		if !isNew {
			continue
		}
		listing := s.scoreAndRecordListings(search, l, marketCache)
		buildNotifications(search, listing, &out)
	}
	return out
}

func (s *Scheduler) persistListings(ctx context.Context, records []storage.ListingRecord) error {
	if err := s.stores.Listings.SaveListings(ctx, records); err != nil {
		s.logger.Error("batch save listings failed", "error", err)
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
		defer cleanupCancel()
		for _, rec := range records {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, rec.Token, rec.ChatID); relErr != nil {
				s.logger.Error("release claim after batch save failure",
					"token", rec.Token, "chat_id", rec.ChatID, "error", relErr)
			}
		}
		return err
	}
	return nil
}

func (s *Scheduler) deliverResults(ctx context.Context, search storage.Search, lang locale.Lang, sr searchResult) bool {
	delivery := s.deliveryFor(ctx, search.ChatID, lang)
	sent := false

	for _, msg := range sr.priceDropMessages {
		if err := delivery.DeliverRaw(ctx, search.ChatID, msg); err != nil {
			if errors.Is(err, notifier.ErrRecipientBlocked) {
				s.logger.Warn("user blocked bot, deactivating",
					"chat_id", search.ChatID,
				)
				if s.stores.Users != nil {
					if err := s.stores.Users.SetUserActive(ctx, search.ChatID, false); err != nil {
						s.logger.Error("set user inactive after block (price drop)",
							"chat_id", search.ChatID,
							"error", err,
						)
					}
				}
				return false
			}
			s.logger.Error("price drop delivery failed",
				"chat_id", search.ChatID,
				"error", err,
			)
		} else {
			sent = true
		}
	}

	if len(sr.newListings) == 0 {
		return sent
	}

	s.observer.RecordListingsFound(len(sr.newListings))

	s.logger.Info("notifying user of new listings",
		"chat_id", search.ChatID,
		"search", search.Name,
		"count", len(sr.newListings),
	)

	if err := delivery.DeliverBatch(ctx, search.ChatID, sr.newListings); err != nil {
		if errors.Is(err, notifier.ErrRecipientBlocked) {
			s.logger.Warn("user blocked bot, deactivating",
				"chat_id", search.ChatID,
			)
			if s.stores.Users != nil {
				if err := s.stores.Users.SetUserActive(ctx, search.ChatID, false); err != nil {
					s.logger.Error("set user inactive after block (batch)",
						"chat_id", search.ChatID,
						"error", err,
					)
				}
			}
		} else {
			s.logger.Error("delivery failed",
				"chat_id", search.ChatID,
				"error", err,
			)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
		defer cleanupCancel()
		for _, l := range sr.newListings {
			if relErr := s.stores.Dedup.ReleaseClaim(cleanupCtx, l.Token, search.ChatID); relErr != nil {
				s.logger.Error("release claim after delivery failure",
					"token", l.Token, "chat_id", search.ChatID, "error", relErr,
				)
			}
		}
	} else {
		s.observer.RecordNotificationSent()
		sent = true
	}
	return sent
}

func (s *Scheduler) processDigests(ctx context.Context) {
	if s.stores.Digests == nil {
		return
	}

	users, err := s.stores.Digests.PendingDigestUsers(ctx)
	if err != nil {
		s.logger.Error("list pending digest users failed", "error", err)
		return
	}

	for _, chatID := range users {
		mode, intervalStr, err := s.stores.Digests.GetDigestMode(ctx, chatID)
		if err != nil {
			s.logger.Error("get digest mode failed", "chat_id", chatID, "error", err)
			continue
		}
		if mode != "digest" {
			// User switched back to instant; flush and send immediately.
			s.flushAndSendDigest(ctx, chatID)
			continue
		}

		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			s.logger.Error("parse digest interval failed",
				"chat_id", chatID,
				"interval", intervalStr,
				"error", err,
			)
			interval = 6 * time.Hour
		}

		lastFlushed, err := s.stores.Digests.DigestLastFlushed(ctx, chatID)
		if err != nil {
			s.logger.Error("get last flushed failed", "chat_id", chatID, "error", err)
			continue
		}

		if time.Since(lastFlushed) >= interval {
			s.flushAndSendDigest(ctx, chatID)
		}
	}
}

func (s *Scheduler) flushAndSendDigest(ctx context.Context, chatID int64) {
	payloads, cutoff, err := s.stores.Digests.PeekDigest(ctx, chatID)
	if err != nil {
		s.logger.Error("peek digest failed", "chat_id", chatID, "error", err)
		return
	}
	if len(payloads) == 0 {
		return
	}

	chatIDStr := fmt.Sprintf("%d", chatID)
	lang := s.userLang(ctx, chatID)
	header := locale.Tf(lang, "fmt_digest_header", len(payloads))
	combined := header + strings.Join(payloads, "\n\n━━━━━━━━━━━━━━━━━━━━\n\n")

	if err := s.notifier.NotifyRaw(ctx, chatIDStr, combined); err != nil {
		s.logger.Error("send digest failed, items preserved for retry",
			"chat_id", chatID,
			"items", len(payloads),
			"error", err,
		)
		return
	}

	if err := s.stores.Digests.AckDigest(ctx, chatID, cutoff); err != nil {
		s.logger.Error("digest ack failed after successful send, items may be resent",
			"chat_id", chatID,
			"cutoff", cutoff,
			"items", len(payloads),
			"error", err,
		)
	}

	s.logger.Info("digest sent",
		"chat_id", chatID,
		"items", len(payloads),
	)
	s.observer.RecordNotificationSent()
}

func (s *Scheduler) processDailyDigests(ctx context.Context) {
	if s.stores.DailyDigests == nil {
		return
	}

	users, err := s.stores.DailyDigests.ListDailyDigestUsers(ctx)
	if err != nil {
		s.logger.Error("list daily digest users failed", "error", err)
		return
	}

	now := time.Now().In(s.loc)

	for _, u := range users {
		targetMinutes := parseTimeOfDayOrZero(u.DigestTime)
		currentMinutes := now.Hour()*60 + now.Minute()

		diff := currentMinutes - targetMinutes
		if diff < 0 {
			diff = -diff
		}
		if diff > 12*60 {
			diff = 24*60 - diff
		}
		if diff > 15 {
			continue
		}

		lastSentLocal := u.LastSent.In(s.loc)
		if lastSentLocal.Year() == now.Year() &&
			lastSentLocal.Month() == now.Month() &&
			lastSentLocal.Day() == now.Day() {
			continue
		}

		s.sendDailyDigest(ctx, u.ChatID)
	}
}

func (s *Scheduler) sendDailyDigest(ctx context.Context, chatID int64) {
	stats, err := s.stores.DailyDigests.DailyStats(ctx, chatID)
	if err != nil {
		s.logger.Error("compute daily stats failed", "chat_id", chatID, "error", err)
		return
	}

	if len(stats) == 0 {
		return
	}

	lang := s.userLang(ctx, chatID)
	msg := notifier.FormatDailyDigest(stats, lang, time.Now().In(s.loc))

	chatIDStr := fmt.Sprintf("%d", chatID)
	if err := s.notifier.NotifyRaw(ctx, chatIDStr, msg); err != nil {
		s.logger.Error("send daily digest failed", "chat_id", chatID, "error", err)
		return
	}

	if err := s.stores.DailyDigests.UpdateDailyDigestLastSent(ctx, chatID); err != nil {
		s.logger.Error("daily digest last-sent update failed after successful send, digest may be resent",
			"chat_id", chatID,
			"error", err,
		)
	}

	s.logger.Info("daily digest sent", "chat_id", chatID, "searches", len(stats))
}

func (s *Scheduler) deactivateExcessSearches(ctx context.Context, chatID int64, maxActive int) {
	if s.stores.Searches == nil {
		return
	}
	searches, err := s.stores.Searches.ListSearches(ctx, chatID)
	if err != nil {
		s.logger.Error("list searches for downgrade failed", "chat_id", chatID, "error", err)
		return
	}
	var active []storage.Search
	for _, sr := range searches {
		if sr.Active {
			active = append(active, sr)
		}
	}
	if len(active) <= maxActive {
		return
	}
	// Keep the oldest (last in the slice since ListSearches orders by created_at DESC), pause the rest.
	for i := 0; i < len(active)-maxActive; i++ {
		if err := s.stores.Searches.SetSearchActive(ctx, active[i].ID, chatID, false); err != nil {
			s.logger.Error("deactivate excess search failed", "chat_id", chatID, "search_id", active[i].ID, "error", err)
		}
	}
	s.logger.Info("deactivated excess searches on downgrade",
		"chat_id", chatID, "paused", len(active)-maxActive, "kept", maxActive)
}

func (s *Scheduler) isUserPremium(ctx context.Context, chatID int64) bool {
	if s.stores.Users == nil {
		return false
	}
	user, err := s.stores.Users.GetUser(ctx, chatID)
	if err != nil || user == nil {
		return false
	}
	if user.Tier != "premium" {
		return false
	}
	if user.TierExpires.IsZero() {
		return false
	}
	return time.Now().Before(user.TierExpires)
}

func (s *Scheduler) processExpiredPremium(ctx context.Context) {
	if s.stores.Users == nil {
		return
	}
	expired, err := s.stores.Users.ListExpiredPremium(ctx)
	if err != nil {
		s.logger.Error("list expired premium users failed", "error", err)
		return
	}
	if len(expired) == 0 {
		return
	}
	s.cfgMu.RLock()
	maxSearches := s.cfg.Telegram.MaxSearches
	s.cfgMu.RUnlock()
	for _, u := range expired {
		if err := s.stores.Users.SetUserTier(ctx, u.ChatID, "free", time.Time{}); err != nil {
			s.logger.Error("downgrade expired premium user failed",
				"chat_id", u.ChatID,
				"error", err,
			)
			continue
		}
		s.deactivateExcessSearches(ctx, u.ChatID, maxSearches)
		s.logger.Info("premium subscription expired, user downgraded to free",
			"chat_id", u.ChatID,
		)
	}
}

func (s *Scheduler) userLang(ctx context.Context, chatID int64) locale.Lang {
	if v, ok := s.langCache.Load(chatID); ok {
		if l, ok := v.(locale.Lang); ok {
			return l
		}
	}
	lang := locale.Hebrew
	if s.stores.Users != nil {
		user, err := s.stores.Users.GetUser(ctx, chatID)
		if err == nil && user != nil && user.Language != "" {
			lang = locale.Lang(user.Language)
		}
	}
	s.langCache.Store(chatID, lang)
	return lang
}

func (s *Scheduler) carName(manufacturerID, modelID int) string {
	if s.carNames == nil {
		return fmt.Sprintf("%d/%d", manufacturerID, modelID)
	}
	mfr := s.carNames.ManufacturerName(manufacturerID)
	mdl := s.carNames.ModelName(manufacturerID, modelID)
	if mfr == "" || mdl == "" {
		return fmt.Sprintf("%d/%d", manufacturerID, modelID)
	}
	return mfr + " " + mdl
}

func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
