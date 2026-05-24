package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dsionov/carwatch/internal/broker"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/percolator"
	"github.com/dsionov/carwatch/internal/pricelist"
	"github.com/dsionov/carwatch/internal/scoring"
	"github.com/dsionov/carwatch/internal/storage"
	"github.com/dsionov/carwatch/internal/telemetry"
)

const (
	fetchTimeout = 60 * time.Second
	// kmEnrichTimeout bounds per-item mileage/city fetches after the list crawl.
	kmEnrichTimeout         = 25 * time.Minute
	maxBackoff              = 4.0
	minBackoff              = 1.0
	pruneInterval           = 24 * time.Hour
	maxRetries              = 3
	retryBaseDelay          = 2 * time.Second
	defaultConcurrency      = 4
	priceHistoryRetention   = 90 * 24 * time.Hour
	listingHistoryRetention = 90 * 24 * time.Hour
	defaultMarketCacheTTL   = 30 * time.Minute
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
	priceListSvc      *pricelist.Service
	publisher         *broker.Publisher
	pipeline          *ListingPipeline
	percolator        *percolator.Percolator
	triggerCh         chan struct{}

	langCache      sync.Map
	digestCache    sync.Map
	digestFailures sync.Map // chatID (int64) -> time.Time of last flush failure
	cycleCount     uint64

	marketCacheMu      sync.RWMutex
	marketCache        *scoring.MarketCache
	marketCacheBuiltAt time.Time
	marketCacheTTL     time.Duration
}

type digestMeta struct {
	mode     string
	interval string
	cachedAt time.Time
}

const digestCacheTTL = 5 * time.Minute

// Stores groups all storage interfaces the scheduler depends on.
type Stores struct {
	Dedup        storage.DedupStore
	Prices       storage.PriceTracker
	Listings     storage.ListingStore
	Searches     storage.SearchStore
	Users        storage.UserStore
	Digests      storage.DigestStore
	Hidden       storage.HiddenListingStore
	Market       storage.MarketStore
	PriceList    storage.PriceListStore
	DailyDigests storage.DailyDigestStore
	CycleLog     storage.CycleLogStore
}

type searchResult struct {
	newListings       []model.Listing
	priceDropMessages []string
	listingRecords    []storage.ListingRecord
	// recordedTokens tracks tokens whose prices were recorded via
	// RecordPrice during this processing pass.  When persistListings
	// fails the scheduler reverts these records so the next cycle does
	// not see stale prices and fire spurious price-drop notifications.
	recordedTokens []string
}

type Options struct {
	Observer         CycleObserver
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
	PriceListStore   storage.PriceListStore
	PriceListHTTP    pricelist.HTTPDoer
	PriceListSvc     *pricelist.Service
	DailyDigestStore storage.DailyDigestStore
	CycleLogStore    storage.CycleLogStore
	MarketCacheTTL   time.Duration
	Publisher        *broker.Publisher
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

	plSvc := opts.PriceListSvc
	if plSvc == nil && opts.PriceListStore != nil && opts.PriceListHTTP != nil {
		plSvc = pricelist.NewService(opts.PriceListStore, opts.PriceListHTTP, logger)
	}

	mcTTL := opts.MarketCacheTTL
	if mcTTL <= 0 {
		mcTTL = defaultMarketCacheTTL
	}

	return &Scheduler{
		cfg:        cfg,
		configPath: opts.ConfigPath,
		fetcher:    f,
		stores: Stores{
			Dedup:        d,
			Prices:       opts.Prices,
			Listings:     opts.ListingStore,
			Searches:     opts.SearchStore,
			Users:        opts.UserStore,
			Digests:      opts.DigestStore,
			Hidden:       opts.HiddenStore,
			Market:       opts.MarketStore,
			PriceList:    opts.PriceListStore,
			DailyDigests: opts.DailyDigestStore,
			CycleLog:     opts.CycleLogStore,
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
		priceListSvc:      plSvc,
		publisher:         opts.Publisher,
		pipeline:          NewListingPipeline(opts.ListingStore, plSvc, logger),
		percolator:        percolator.New(),
		triggerCh:         make(chan struct{}, 1),
		marketCacheTTL:    mcTTL,
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

func (s *Scheduler) deliveryFor(ctx context.Context, chatID int64, lang locale.Lang, searchID int64, searchName string, log *slog.Logger) DeliveryStrategy {
	if s.stores.Digests != nil {
		var mode string
		needFetch := true
		if v, ok := s.digestCache.Load(chatID); ok {
			if dm, ok := v.(digestMeta); ok && time.Since(dm.cachedAt) < digestCacheTTL {
				mode = dm.mode
				needFetch = false
			}
		}
		if needFetch {
			m, interval, err := s.stores.Digests.GetDigestMode(ctx, chatID)
			if err != nil {
				if !errors.Is(err, storage.ErrNotFound) {
					log.Error("get digest mode failed", "chat_id", chatID, "error", err)
				}
			} else {
				mode = m
				s.digestCache.Store(chatID, digestMeta{mode: m, interval: interval, cachedAt: time.Now()})
			}
		}
		if mode == "digest" {
			return NewDigestDelivery(s.stores.Digests, lang)
		}
	}
	opts := []func(*InstantDelivery){WithLogger(log), WithSearchContext(searchID, searchName)}
	if s.publisher != nil {
		opts = append(opts, WithPublisher(s.publisher))
	}
	return NewInstantDelivery(s.notifier, lang, opts...)
}

func (s *Scheduler) fetcherForSource(source string) fetcher.Fetcher {
	if s.fetcherFactory != nil {
		if f, ok := s.fetcherFactory.Get(source); ok {
			return f
		}
	}
	return s.fetcher
}

func (s *Scheduler) fetchWithRetryUsing(ctx context.Context, f fetcher.Fetcher, params model.SourceParams, log *slog.Logger) ([]model.RawListing, error) {
	var lastErr error
	for attempt := range maxRetries {
		listings, err := f.Fetch(ctx, params)
		if err == nil {
			return listings, nil
		}
		lastErr = err

		if errors.Is(err, fetcher.ErrPartialResults) && len(listings) > 0 {
			log.Warn("partial results (some pages failed)",
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
			log.Warn("fetch failed, retrying",
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

func (s *Scheduler) runMultiTenantCycle(ctx context.Context) error {
	s.cycleCount++
	cycle := s.cycleCount
	cycleStart := time.Now()

	s.logger.Info("checking for new listings", "scan", cycle)

	s.langCache.Range(func(k, _ any) bool { s.langCache.Delete(k); return true })
	s.digestCache.Range(func(k, _ any) bool { s.digestCache.Delete(k); return true })
	if s.priceListSvc != nil {
		s.priceListSvc.ResetCycleCounter()
	}

	searches, err := s.stores.Searches.ListAllActiveSearches(ctx)
	if err != nil {
		return fmt.Errorf("load searches: %w", err)
	}

	s.pruneOldData(ctx)
	s.processExpiredPremium(ctx)

	if len(searches) == 0 {
		s.backfillUnenrichedListings(ctx)
		s.logger.Info("scan complete (no active searches)", "scan", cycle, "elapsed", time.Since(cycleStart).Round(time.Millisecond))
		s.observer.RecordSuccess()
		return nil
	}

	// Phase 3: load all searches into the percolator for reverse matching.
	s.percolator.Load(searches)

	marketCache := s.getOrBuildMarketCache(ctx)
	s.logger.Info("active searches loaded",
		"scan", cycle,
		"searches", len(searches),
	)

	// Fetch the global feed (empty SourceParams = no manufacturer/model filter).
	stats, fetchErr := s.fetchGlobalAndMatch(ctx, searches, marketCache)

	if s.catalogIngester != nil {
		s.catalogIngester.Flush(ctx)
	}

	s.processDigests(ctx)
	s.processDailyDigests(ctx)
	s.backfillUnenrichedListings(ctx)

	s.logger.Info("scan complete",
		"scan", cycle,
		"elapsed", time.Since(cycleStart).Round(time.Millisecond),
		"searches", len(searches),
		"listings_checked", stats.listingsFetched,
		"new_matches", stats.newListings,
		"notifications_sent", stats.notificationsSent,
	)

	if telemetry.SchedulerCycles != nil {
		telemetry.SchedulerCycles.Add(ctx, 1)
	}
	if telemetry.ListingsFetched != nil {
		telemetry.ListingsFetched.Add(ctx, int64(stats.listingsFetched))
	}
	if telemetry.ListingsMatched != nil {
		telemetry.ListingsMatched.Add(ctx, int64(stats.newListings))
	}

	status := "ok"
	errMsg := ""
	if fetchErr != nil {
		status = "error"
		errMsg = fetchErr.Error()
		s.observer.RecordError()
	} else {
		s.observer.RecordSuccess()
	}

	s.writeCycleLog(ctx, storage.CycleLogEntry{
		StartedAt:       cycleStart,
		DurationMs:      int(time.Since(cycleStart).Milliseconds()),
		Searches:        len(searches),
		ListingsFetched: stats.listingsFetched,
		ListingsMatched: stats.newListings,
		Notifications:   stats.notificationsSent,
		ErrorMessage:    errMsg,
		Status:          status,
	})

	if fetchErr != nil {
		return fetchErr
	}
	return nil
}

func (s *Scheduler) writeCycleLog(ctx context.Context, entry storage.CycleLogEntry) {
	if s.stores.CycleLog == nil {
		return
	}
	if err := s.stores.CycleLog.WriteCycleLog(ctx, entry); err != nil {
		s.logger.Error("write cycle log failed", "error", err)
	}
}

// fetchGlobalAndMatch fetches the global Yad2 feed once, enriches it, then
// uses the percolator to match each listing against all active searches.
// Per-user dedup, pipeline processing, and notification delivery happen
// per match.
func (s *Scheduler) fetchGlobalAndMatch(ctx context.Context, searches []storage.Search, marketCache *scoring.MarketCache) (cycleStats, error) {
	var stats cycleStats

	// 1. Fetch global feed (empty SourceParams).
	globalParams := model.SourceParams{}
	activeFetcher := s.fetcherForSource("yad2")

	fetchCtx, cancelFetch := context.WithTimeout(ctx, fetchTimeout)
	defer cancelFetch()

	fetchStart := time.Now()
	raw, err := s.fetchWithRetryUsing(fetchCtx, activeFetcher, globalParams, s.logger)
	s.observer.RecordFetch("yad2", time.Since(fetchStart), err)
	if err != nil {
		return stats, fmt.Errorf("global feed fetch failed: %w", err)
	}
	stats.listingsFetched = len(raw)

	// 2. Catalog ingestion.
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

	// 3. KM enrichment on the global feed.
	prefilled := false
	if s.kmEnricher != nil {
		enrichCtx, cancelEnrich := context.WithTimeout(ctx, kmEnrichTimeout)
		enriched := s.kmEnricher.Enrich(enrichCtx, raw)
		cancelEnrich()
		if enriched > 0 {
			s.logger.Info("mileage data enriched",
				"enriched", enriched,
				"total", len(raw),
			)
			if s.stores.Listings != nil {
				s.backfillEnrichedListings(ctx, raw)
			}
		}
		if s.stores.Listings != nil {
			s.prefillFromDB(ctx, raw)
			prefilled = true
		}
	}

	// 4. Build a per-search accumulator to collect results across listings.
	type searchAccum struct {
		search storage.Search
		result searchResult
		lang   locale.Lang
	}
	accums := make(map[int64]*searchAccum, len(searches))

	// 5. Percolator match: for each listing, find matching searches.
	hiddenCache := make(map[int64]map[string]bool)

	for i := range raw {
		matches := s.percolator.Match(raw[i])
		if len(matches) == 0 {
			continue
		}

		for _, m := range matches {
			acc, ok := accums[m.SearchID]
			if !ok {
				lang := s.userLang(ctx, m.ChatID)
				acc = &searchAccum{
					search: m.Search,
					lang:   lang,
				}
				accums[m.SearchID] = acc
			}

			// Per-user hidden listing check (cached per chatID per cycle).
			hidden, cached := hiddenCache[m.ChatID]
			if !cached {
				hidden = s.loadHiddenTokens(ctx, m.ChatID)
				hiddenCache[m.ChatID] = hidden
			}
			if len(hidden) > 0 && hidden[raw[i].Token] {
				continue
			}

			// Per-user, per-search dedup.
			isNew, ok := s.deduplicateListings(ctx, raw[i].Token, m.ChatID, m.SearchID, s.logger)
			if !ok {
				continue
			}

			// Price drop detection.
			if s.tryPriceDropListing(ctx, m.Search, raw[i], acc.lang, marketCache, &acc.result, s.logger) {
				continue
			}

			if !isNew {
				continue
			}

			// Collect the raw listing for pipeline processing.
			acc.result.newListings = append(acc.result.newListings, model.Listing{RawListing: raw[i]})
		}
	}

	// 6. Run pipeline and deliver results per search.
	for _, acc := range accums {
		// Convert collected raw listings through the pipeline.
		if len(acc.result.newListings) > 0 {
			rawForPipeline := make([]model.RawListing, len(acc.result.newListings))
			for i, l := range acc.result.newListings {
				rawForPipeline[i] = l.RawListing
			}
			params := ProcessParamsFromSearch(acc.search, marketCache)
			params.SkipPrefill = prefilled
			pr := s.pipeline.Process(ctx, rawForPipeline, params)
			acc.result.newListings = pr.Listings
			acc.result.listingRecords = pr.Records
		}

		// Persist listings.
		persistOK := true
		if s.stores.Listings != nil && len(acc.result.listingRecords) > 0 {
			searchLog := s.logger.With("search_id", acc.search.ID, "chat_id", acc.search.ChatID)
			if err := s.persistListings(ctx, acc.result.listingRecords, searchLog); err != nil {
				persistOK = false
			} else {
				s.invalidateMarketCache()
			}
		}
		if !persistOK {
			searchLog := s.logger.With("search_id", acc.search.ID, "chat_id", acc.search.ChatID)
			acc.result.newListings = nil
			acc.result.listingRecords = nil
			s.revertPriceRecords(ctx, acc.result.recordedTokens, searchLog)
		}

		stats.newListings += len(acc.result.newListings)

		// Deliver notifications.
		searchLog := s.logger.With(
			"search_id", acc.search.ID,
			"chat_id", acc.search.ChatID,
			"search_name", acc.search.Name,
		)
		delivered := s.deliverResults(ctx, acc.search, acc.lang, acc.result, searchLog)
		if delivered {
			stats.notificationsSent++
		}
	}

	return stats, nil
}

func (s *Scheduler) getOrBuildMarketCache(ctx context.Context) *scoring.MarketCache {
	if s.stores.Market == nil {
		return nil
	}

	// Fast path: check if the cached value is still fresh.
	s.marketCacheMu.RLock()
	if s.marketCache != nil && time.Since(s.marketCacheBuiltAt) < s.marketCacheTTL {
		mc := s.marketCache
		s.marketCacheMu.RUnlock()
		s.logger.Debug("reusing cached market data",
			"age", time.Since(s.marketCacheBuiltAt).Round(time.Second).String())
		return mc
	}
	s.marketCacheMu.RUnlock()

	// Slow path: rebuild under write lock.
	s.marketCacheMu.Lock()
	defer s.marketCacheMu.Unlock()

	// Double-check after acquiring the write lock.
	if s.marketCache != nil && time.Since(s.marketCacheBuiltAt) < s.marketCacheTTL {
		return s.marketCache
	}

	mc := s.buildMarketCache(ctx)
	if mc != nil {
		s.marketCache = mc
		s.marketCacheBuiltAt = time.Now()
	}
	return s.marketCache
}

func (s *Scheduler) buildMarketCache(ctx context.Context) *scoring.MarketCache {
	if err := s.stores.Market.RefreshMarketMedians(ctx); err != nil {
		s.logger.Error("refresh market medians failed", "error", err)
	}

	rows, err := s.stores.Market.LoadMarketMedians(ctx)
	if err != nil {
		s.logger.Error("load market medians failed, keeping previous cache", "error", err)
		return nil
	}

	entries := make([]scoring.MedianEntry, len(rows))
	for i, r := range rows {
		entries[i] = scoring.MedianEntry{
			Manufacturer: r.Manufacturer,
			Model:        r.Model,
			Year:         r.Year,
			MedianPrice:  r.MedianPrice,
			MedianKm:     r.MedianKm,
			CohortSize:   r.CohortSize,
		}
	}
	return scoring.NewMarketCacheFromMedians(entries)
}

func (s *Scheduler) invalidateMarketCache() {
	s.marketCacheMu.Lock()
	s.marketCache = nil
	s.marketCacheBuiltAt = time.Time{}
	s.marketCacheMu.Unlock()
}

type cycleStats struct {
	listingsFetched   int
	newListings       int
	notificationsSent int
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

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
