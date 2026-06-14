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
	"github.com/dsionov/carwatch/internal/cwlog"
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
	fetchTimeout            = 60 * time.Second
	pruneInterval           = 24 * time.Hour
	maxRetries              = 3
	retryBaseDelay          = 2 * time.Second
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

type Scheduler struct {
	cfgMu           sync.RWMutex
	cfg             *config.Config
	configPath      string
	fetcher         fetcher.Fetcher
	targetedFetcher fetcher.Fetcher // bypasses circuit breaker for per-model fetches
	stores          Stores
	notifier        notifier.Notifier
	logger          *slog.Logger
	loc             *time.Location
	lastPruneTime   time.Time
	observer        CycleObserver
	fetcherFactory  *fetcher.Factory
	catalogIngester CatalogIngester
	carNames        CarNameResolver
	priceListSvc    *pricelist.Service
	publisher       *broker.Publisher
	enrichPublisher *broker.EnrichPublisher
	pipeline        *ListingPipeline
	percolator      *percolator.Percolator
	triggerCh       chan struct{}

	langCache      sync.Map
	digestCache    sync.Map
	digestFailures sync.Map // chatID (int64) -> time.Time of last flush failure
	cycleCount     uint64

	// bgWG tracks long-lived background goroutines (e.g. the market-cache
	// refresh loop) so Run can wait for them to exit before returning. Without
	// this, a returning Run lets the caller close the store while a refresh is
	// still mid-query.
	bgWG sync.WaitGroup

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
	Dedup            storage.DedupStore
	Prices           storage.PriceTracker
	Listings         storage.ListingStore
	Searches         storage.SearchStore
	Users            storage.UserStore
	Digests          storage.DigestStore
	Hidden           storage.HiddenListingStore
	Market           storage.MarketStore
	PriceList        storage.PriceListStore
	DailyDigests     storage.DailyDigestStore
	CycleLog         storage.CycleLogStore
	SearchCycleStats storage.SearchCycleStatsStore
}

type searchResult struct {
	newListings       []model.Listing
	priceDropMessages []string
	listingRecords    []storage.ListingRecord
}

type Options struct {
	Observer              CycleObserver
	Prices                storage.PriceTracker
	ConfigPath            string
	FetcherFactory        *fetcher.Factory
	TargetedFetcher       fetcher.Fetcher // bypasses circuit breaker for per-model fetches
	ListingStore          storage.ListingStore
	SearchStore           storage.SearchStore
	UserStore             storage.UserStore
	DigestStore           storage.DigestStore
	HiddenStore           storage.HiddenListingStore
	CatalogIngester       CatalogIngester
	CarNames              CarNameResolver
	MarketStore           storage.MarketStore
	PriceListStore        storage.PriceListStore
	PriceListHTTP         pricelist.HTTPDoer
	PriceListSvc          *pricelist.Service
	DailyDigestStore      storage.DailyDigestStore
	CycleLogStore         storage.CycleLogStore
	SearchCycleStatsStore storage.SearchCycleStatsStore
	MarketCacheTTL        time.Duration
	Publisher             *broker.Publisher
	EnrichPublisher       *broker.EnrichPublisher
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

	tf := opts.TargetedFetcher
	if tf == nil {
		tf = f
	}

	return &Scheduler{
		cfg:             cfg,
		configPath:      opts.ConfigPath,
		fetcher:         f,
		targetedFetcher: tf,
		stores: Stores{
			Dedup:            d,
			Prices:           opts.Prices,
			Listings:         opts.ListingStore,
			Searches:         opts.SearchStore,
			Users:            opts.UserStore,
			Digests:          opts.DigestStore,
			Hidden:           opts.HiddenStore,
			Market:           opts.MarketStore,
			PriceList:        opts.PriceListStore,
			DailyDigests:     opts.DailyDigestStore,
			CycleLog:         opts.CycleLogStore,
			SearchCycleStats: opts.SearchCycleStatsStore,
		},
		notifier:        n,
		logger:          logger,
		loc:             loc,
		observer:        obs,
		fetcherFactory:  opts.FetcherFactory,
		catalogIngester: opts.CatalogIngester,
		carNames:        opts.CarNames,
		priceListSvc:    plSvc,
		publisher:       opts.Publisher,
		enrichPublisher: opts.EnrichPublisher,
		pipeline:        NewListingPipeline(opts.ListingStore, plSvc, logger),
		percolator:      percolator.New(),
		triggerCh:       make(chan struct{}, 1),
		marketCacheTTL:  mcTTL,
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
	s.logger.Info("scheduler started, entering polling loop",
		"check_interval", logInterval.String(),
		"jitter", logJitter.String(),
	)

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)

	// Wait for background goroutines (market-cache refresh) to exit before
	// returning, so the caller does not close the store out from under them.
	defer s.bgWG.Wait()
	s.startBackgroundTasks(ctx)

	cycle := s.runMultiTenantCycle

	if s.isActiveHours() {
		if err := cycle(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			s.logger.Error("initial scheduler cycle failed on startup", "error", err)
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
				s.logger.Info("outside configured active hours, sleeping until polling window opens",
					"sleep", sleepUntil.Round(time.Minute).String(),
				)
				delay = sleepUntil
			}
		}

		s.logger.Info("scheduling next scan cycle",
			"in", delay.Round(time.Second).String(),
			"at", time.Now().Add(delay).In(s.loc).Format("15:04:05"))

		timer.Reset(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			s.logger.Info("scheduler received shutdown signal, stopping gracefully")
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
			s.logger.Info("scheduler cycle triggered by manual API request")
		case <-timer.C:
		}

		if !s.isActiveHours() {
			continue
		}

		if err := cycle(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			s.logger.Error("scheduler cycle failed, will retry on next interval", "error", err)
		}
	}
}

// startBackgroundTasks launches long-lived background goroutines tracked by
// bgWG. Currently this is the periodic market-cache refresh. It is a no-op when
// no market store is configured.
func (s *Scheduler) startBackgroundTasks(ctx context.Context) {
	if s.stores.Market == nil {
		return
	}
	s.bgWG.Add(1)
	go func() {
		defer s.bgWG.Done()
		s.marketRefreshLoop(ctx)
	}()
}

// marketRefreshLoop refreshes the market view immediately and then on every
// marketCacheTTL tick until the context is cancelled.
func (s *Scheduler) marketRefreshLoop(ctx context.Context) {
	s.refreshMarketView(ctx)
	ticker := time.NewTicker(s.marketCacheTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshMarketView(ctx)
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
			log.WarnContext(ctx, "received partial results from source, some pages failed to fetch",
				"car", s.carName(params.Manufacturer, params.Model),
				"listings_returned", len(listings),
				"error", err,
			)
			return listings, nil
		}

		if errors.Is(err, fetcher.ErrChallenge) || errors.Is(err, fetcher.ErrRateLimited) || errors.Is(err, fetcher.ErrCircuitOpen) || errors.Is(err, context.Canceled) {
			return nil, err
		}

		if attempt < maxRetries-1 {
			delay := retryBaseDelay * (1 << attempt)
			log.WarnContext(ctx, "fetch attempt failed, retrying with exponential backoff",
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
	base := interval
	if jitterCfg > 0 {
		offset := time.Duration(rand.Int64N(int64(2*jitterCfg))) - jitterCfg
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
	s.logger.Info("received SIGHUP signal, reloading configuration from disk", "path", s.configPath)
	newCfg, err := config.Load(s.configPath)
	if err != nil {
		s.logger.Error("failed to reload configuration, keeping current config", "error", err)
		return
	}
	loc, err := time.LoadLocation(newCfg.Polling.Timezone)
	if err != nil {
		s.logger.Error("failed to reload configuration, invalid timezone setting", "timezone", newCfg.Polling.Timezone, "error", err)
		return
	}
	s.cfgMu.Lock()
	s.cfg = newCfg
	s.loc = loc
	s.cfgMu.Unlock()
	s.logger.Info("configuration reloaded successfully")
}

func (s *Scheduler) runMultiTenantCycle(ctx context.Context) error {
	s.cycleCount++
	cycle := s.cycleCount
	cycleStart := time.Now()
	ctx = cwlog.WithCycleID(ctx, cycle)

	s.logger.InfoContext(ctx, "starting scheduler cycle, loading active searches", "scan", cycle)

	s.langCache.Clear()
	s.digestCache.Clear()
	if s.priceListSvc != nil {
		s.priceListSvc.ResetCycleCounter()
	}

	if s.stores.Users != nil {
		if n, err := s.stores.Users.ReactivateUsersWithSearches(ctx); err != nil {
			s.logger.WarnContext(ctx, "failed to reactivate users with active searches", "error", err)
		} else if n > 0 {
			s.logger.InfoContext(ctx, "reactivated users who had active searches but were inactive", "count", n)
		}
	}

	dbStart := time.Now()
	searches, err := s.stores.Searches.ListAllActiveSearches(ctx)
	if err != nil {
		return fmt.Errorf("load searches: %w", err)
	}
	searchLoadMs := time.Since(dbStart).Milliseconds()

	s.pruneOldData(ctx)
	s.processExpiredPremium(ctx)

	if len(searches) == 0 {
		s.backfillUnenrichedListings(ctx)
		s.logger.InfoContext(ctx, "scheduler cycle completed with no active searches", "scan", cycle, "elapsed", time.Since(cycleStart).Round(time.Millisecond))
		s.observer.RecordSuccess()
		return nil
	}

	// Phase 3: load all searches into the percolator for reverse matching.
	s.percolator.Load(searches)

	marketCache := s.getOrBuildMarketCache(ctx)
	s.logger.InfoContext(ctx, "loaded active searches into percolator for reverse matching",
		"scan", cycle,
		"searches", len(searches),
		"db_duration_ms", searchLoadMs,
	)

	stats, fetchErr := s.fetchAndMatch(ctx, searches, marketCache)

	if s.catalogIngester != nil {
		s.catalogIngester.Flush(ctx)
	}

	s.processDigests(ctx)
	s.processDailyDigests(ctx)
	s.backfillUnenrichedListings(ctx)

	s.logger.InfoContext(ctx, "scheduler cycle completed",
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
		s.logger.Error("failed to write scheduler cycle log entry", "error", err)
	}
}

// fetchTargetedListings fetches listings for each active search using the
// search's full filter set (manufacturer, model, year, price, km, etc.).
// This ensures Yad2 pre-filters results so every page contains relevant
// listings, giving much better coverage than the unfiltered global feed.
func (s *Scheduler) fetchTargetedListings(ctx context.Context, searches []storage.Search, raw []model.RawListing, f fetcher.Fetcher) []model.RawListing {
	// Build the dedup set from existing tokens.
	seen := make(map[string]struct{}, len(raw))
	for _, l := range raw {
		seen[l.Token] = struct{}{}
	}

	// Track which param combinations we've already fetched so searches
	// with identical filters don't produce redundant API calls.
	fetchedKeys := make(map[string]struct{})

	var fetched, added int
	for _, sr := range searches {
		if sr.Manufacturer == 0 || sr.Model == 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return raw
		default:
		}

		params := model.SourceParamsFromSearch(&sr)
		key := fetcher.CacheKeyFor(params)
		if _, done := fetchedKeys[key]; done {
			continue
		}
		fetchedKeys[key] = struct{}{}

		targetCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
		targeted, err := s.fetchWithRetryUsing(targetCtx, f, params, s.logger)
		cancel()

		if err != nil {
			if errors.Is(err, context.Canceled) {
				return raw
			}
			delay := 3 * time.Second
			if errors.Is(err, fetcher.ErrRateLimited) {
				delay = 5 * time.Second
			}
			s.logger.WarnContext(ctx, "targeted fetch failed, skipping search",
				"search_id", sr.ID, "search_name", sr.Name,
				"car", s.carName(sr.Manufacturer, sr.Model),
				"error", err, "cooldown", delay.String())
			select {
			case <-ctx.Done():
				return raw
			case <-time.After(delay):
			}
			continue
		}
		fetched++

		for _, l := range targeted {
			if _, dup := seen[l.Token]; dup {
				continue
			}
			seen[l.Token] = struct{}{}
			raw = append(raw, l)
			added++
		}

		// Pause between targeted fetches to avoid Yad2 rate limiting.
		select {
		case <-ctx.Done():
			return raw
		case <-time.After(3 * time.Second):
		}
	}

	if fetched > 0 {
		s.logger.InfoContext(ctx, "fetched targeted listings for active searches",
			"searches_fetched", fetched,
			"unique_params", len(fetchedKeys),
			"new_listings_added", added,
		)
	}
	return raw
}

// fetchAndMatch fetches listings for each active search via targeted
// per-model API calls, then matches each listing against all active
// searches via the percolator. Per-user dedup, pipeline processing,
// and notification delivery happen per match.
func (s *Scheduler) fetchAndMatch(ctx context.Context, searches []storage.Search, marketCache *scoring.MarketCache) (cycleStats, error) {
	var stats cycleStats

	fetchStart := time.Now()
	raw := s.fetchTargetedListings(ctx, searches, nil, s.targetedFetcher)
	fetchDuration := time.Since(fetchStart)
	stats.listingsFetched = len(raw)
	s.observer.RecordFetch("yad2", fetchDuration, nil)

	s.logger.Info("targeted listings fetched",
		"scan", s.cycleCount,
		"source", "yad2",
		"listings", len(raw),
		"active_searches", len(searches),
		"duration_ms", fetchDuration.Milliseconds(),
	)

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

	// 3. Prefill from DB so listings with known km/city/image from prior
	// cycles get that data before matching.
	prefilled := false
	if s.stores.Listings != nil {
		prefilled = s.prefillFromDB(ctx, raw)
	}

	// 4. Build a per-search accumulator to collect results across listings.
	type searchAccum struct {
		search storage.Search
		result searchResult
		lang   locale.Lang
	}
	accums := make(map[int64]*searchAccum, len(searches))

	// Per-search cycle stats for observability.
	type searchCycleCounter struct {
		matched    int // percolator hits before dedup
		kmFiltered int // held back by km filter
	}
	searchCounters := make(map[int64]*searchCycleCounter)

	// Deferred price drop candidates: we read prices during matching
	// (to detect changes) but defer notification formatting until after
	// enrichment so messages include enriched km/city data.
	type priceDropCandidate struct {
		rawIdx   int
		searchID int64
		oldPrice int
	}
	var priceDropCandidates []priceDropCandidate

	// Price observations are staged here during matching and durably
	// recorded only after the listing outcome is settled (see
	// flushPendingPrices). persistedTokens / persistFailedTokens track
	// per-token persist outcomes across all searches in this cycle.
	pendingPrices := make(map[string]int)
	persistedTokens := make(map[string]bool)
	persistFailedTokens := make(map[string]bool)

	// 5. Percolator match: for each listing, find matching searches.
	hiddenCache := make(map[int64]map[string]bool)
	matchedIndices := make(map[int][]int64)
	type priceResult struct {
		oldPrice int
		changed  bool
		err      bool
	}
	priceCache := make(map[string]priceResult)

	for i := range raw {
		matches := s.percolator.Match(raw[i])
		if len(matches) == 0 {
			continue
		}
		if _, seen := matchedIndices[i]; !seen {
			ids := make([]int64, 0, len(matches))
			for _, m := range matches {
				ids = append(ids, m.SearchID)
			}
			matchedIndices[i] = ids
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

			ctr, ok := searchCounters[m.SearchID]
			if !ok {
				ctr = &searchCycleCounter{}
				searchCounters[m.SearchID] = ctr
			}
			ctr.matched++

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
			matchLog := s.logger.With("search_id", m.SearchID, "chat_id", m.ChatID, "search_name", m.SearchName)
			isNew, ok := s.deduplicateListings(ctx, raw[i].Token, m.ChatID, m.SearchID, matchLog)
			if !ok {
				continue
			}

			// Read the last recorded price to detect changes; the durable
			// write is staged and flushed only after the listing outcome
			// is settled (flushPendingPrices). Cache per token so
			// subsequent matches for the same listing reuse the read.
			if s.stores.Prices != nil && raw[i].Price > 0 {
				pr, cached := priceCache[raw[i].Token]
				if !cached {
					prevPrice, found, err := s.stores.Prices.PeekPrice(ctx, raw[i].Token)
					if err != nil {
						matchLog.ErrorContext(ctx, "failed to read last recorded listing price",
							"token", raw[i].Token, "error", err.Error())
						pr = priceResult{err: true}
					} else {
						pr = priceResult{oldPrice: prevPrice, changed: found && raw[i].Price != prevPrice}
						if !found || pr.changed {
							pendingPrices[raw[i].Token] = raw[i].Price
						}
					}
					priceCache[raw[i].Token] = pr
				}
				if !pr.err && pr.changed && raw[i].Price < pr.oldPrice {
					priceDropCandidates = append(priceDropCandidates, priceDropCandidate{
						rawIdx: i, searchID: m.SearchID, oldPrice: pr.oldPrice,
					})
					continue
				}
			}

			if !isNew {
				continue
			}

			// Collect the raw listing for pipeline processing.
			acc.result.newListings = append(acc.result.newListings, model.Listing{RawListing: raw[i]})
		}
	}

	// 5a. Publish enrichment requests to the Redis stream for each matched
	// listing that still needs enrichment data. The enricher worker
	// processes these idempotently (skip if already enriched).
	if s.enrichPublisher != nil && len(matchedIndices) > 0 {
		published := 0
		for idx, searchIDs := range matchedIndices {
			l := raw[idx]
			if l.Km <= 0 || l.City == "" || l.ImageURL == "" {
				req := broker.EnrichRequest{
					Token:      l.Token,
					Priority:   1,
					SearchIDs:  searchIDs,
					Source:     "scheduler",
					EnqueuedAt: time.Now().UTC().Format(time.RFC3339),
				}
				if err := s.enrichPublisher.PublishEnrich(ctx, req); err != nil {
					s.logger.WarnContext(ctx, "failed to publish enrichment request to stream",
						"token", l.Token, "car", l.Manufacturer+" "+l.Model, "error", err)
				} else {
					published++
				}
			}
		}
		if published > 0 {
			s.logger.InfoContext(ctx, "published enrichment requests for matched listings",
				"published", published, "total_matched", len(matchedIndices))
		}
	}

	// 5b. Process deferred price drops.
	for _, pd := range priceDropCandidates {
		acc := accums[pd.searchID]
		if acc == nil {
			continue
		}
		l := raw[pd.rawIdx]
		s.logger.InfoContext(ctx, "detected price drop for matched listing, preparing to notify user",
			"token", l.Token, "old_price", pd.oldPrice, "new_price", l.Price,
			"price_change", l.Price-pd.oldPrice,
			"manufacturer", l.Manufacturer, "model", l.Model, "year", l.Year,
		)
		listing := model.Listing{RawListing: l, SearchName: acc.search.Name}
		fp := scoring.FitnessParams{
			Price: l.Price, Km: l.Km, Hand: l.Hand, Year: l.Year,
			EngineVolume: l.EngineVolume, PriceMax: acc.search.PriceMax,
			MaxKm: acc.search.MaxKm, MaxHand: acc.search.MaxHand,
			YearMin: acc.search.YearMin, YearMax: acc.search.YearMax,
			EngineMinCC: acc.search.EngineMinCC,
		}
		if marketCache != nil {
			if median, medKm, _, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year); ok {
				fp.MedianPrice = median
				fp.MedianKm = medKm
			}
		}
		listing.FitnessScore = scoring.FitnessScore(fp)
		acc.result.priceDropMessages = append(acc.result.priceDropMessages, notifier.FormatPriceDrop(listing, pd.oldPrice, acc.lang))
		if s.stores.Listings != nil {
			rec := storage.ListingRecord{
				Token: l.Token, ChatID: acc.search.ChatID, SearchID: acc.search.ID, SearchName: acc.search.Name,
				Manufacturer: l.Manufacturer, Model: l.Model, SubModel: l.SubModel,
				SubModelID: l.SubModelID,
				Year:       l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
				City: l.City, PageLink: l.PageLink, ImageURL: l.ImageURL,
				EngineVolume: l.EngineVolume, HorsePower: l.HorsePower,
				EngineType: l.EngineType, GearBox: l.GearBox, Description: l.Description,
				IsCommercial: l.Commercial,
				FitnessScore: &listing.FitnessScore, FirstSeenAt: time.Now(),
			}
			if !l.CreatedAt.IsZero() {
				t := l.CreatedAt
				rec.PostedAt = &t
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
				s.logger.ErrorContext(ctx, "failed to persist price-drop listing to history",
					"token", l.Token, "error", err.Error())
			}
		}
	}

	// Build a token→index map so the pipeline reads enriched data from raw.
	rawByToken := make(map[string]int, len(raw))
	for i := range raw {
		rawByToken[raw[i].Token] = i
	}

	// 6. Run pipeline and deliver results per search.
	for _, acc := range accums {
		searchCtx := cwlog.WithSearch(ctx, acc.search.ID, acc.search.ChatID)

		// Convert collected raw listings through the pipeline, using the
		// enriched version from raw (not the stale copy in newListings).
		if len(acc.result.newListings) > 0 {
			rawForPipeline := make([]model.RawListing, len(acc.result.newListings))
			for i, l := range acc.result.newListings {
				if idx, ok := rawByToken[l.Token]; ok {
					rawForPipeline[i] = raw[idx]
				} else {
					rawForPipeline[i] = l.RawListing
				}
			}

			// Process ALL matched listings through the pipeline so they
			// get persisted to listing_history. The enricher needs the
			// rows to exist before it can backfill km/city data.
			if len(rawForPipeline) == 0 {
				acc.result.newListings = nil
			} else {
				params := ProcessParamsFromSearch(acc.search, marketCache)
				params.SkipPrefill = prefilled
				pr := s.pipeline.Process(searchCtx, rawForPipeline, params)
				acc.result.newListings = pr.Listings
				acc.result.listingRecords = pr.Records
			}

			// Post-enrichment MaxKm filter: suppress notifications for
			// listings whose km exceeds the search cap or is still
			// unknown (pending enrichment). Listings are still persisted
			// above so the enricher can backfill km data.
			if acc.search.MaxKm > 0 && len(acc.result.newListings) > 0 {
				var deliverable []model.Listing
				for _, l := range acc.result.newListings {
					if l.Km <= 0 || l.Km > acc.search.MaxKm {
						reason := "km_exceeded"
						if l.Km <= 0 {
							reason = "km_pending_enrichment"
						}
						if relErr := s.stores.Dedup.ReleaseClaim(ctx, l.Token, acc.search.ChatID, acc.search.ID); relErr != nil {
							s.logger.ErrorContext(searchCtx,
								"failed to release dedup claim for km-filtered listing",
								"token", l.Token, "reason", reason, "error", relErr.Error())
						}
						if ctr := searchCounters[acc.search.ID]; ctr != nil {
							ctr.kmFiltered++
						}
						continue
					}
					deliverable = append(deliverable, l)
				}
				if len(deliverable) < len(acc.result.newListings) {
					s.logger.InfoContext(searchCtx,
						"filtered listings by km limit after enrichment",
						"search_name", acc.search.Name,
						"chat_id", acc.search.ChatID,
						"before", len(acc.result.newListings),
						"after", len(deliverable),
						"max_km", acc.search.MaxKm,
					)
					acc.result.newListings = deliverable
				}
			}
		}

		// Persist listings. Per-token outcomes feed flushPendingPrices: a
		// staged price is only written durably once its listing made it
		// into history somewhere.
		searchLog := s.logger.With("chat_id", acc.search.ChatID, "search_name", acc.search.Name)
		persistOK := true
		if s.stores.Listings != nil && len(acc.result.listingRecords) > 0 {
			if err := s.persistListings(searchCtx, acc.result.listingRecords, searchLog); err != nil {
				persistOK = false
				for _, rec := range acc.result.listingRecords {
					persistFailedTokens[rec.Token] = true
				}
			} else {
				s.invalidateMarketCache()
				for _, rec := range acc.result.listingRecords {
					persistedTokens[rec.Token] = true
				}
			}
		}
		if !persistOK {
			acc.result.newListings = nil
			acc.result.listingRecords = nil
		}

		stats.newListings += len(acc.result.newListings)

		// Enrich grace window: defer notification for unenriched listings
		// that were just seen, giving the enricher time to fill in km/city/image.
		graceSec := s.cfg.Polling.EnrichGraceSeconds
		if graceSec <= 0 {
			graceSec = 60
		}
		if graceSec > 0 && len(acc.result.newListings) > 0 {
			var ready []model.Listing
			deferred := 0
			for _, l := range acc.result.newListings {
				unenriched := l.Km <= 0 || l.City == "" || l.ImageURL == ""
				justSeen := l.CreatedAt.IsZero() || time.Since(l.CreatedAt) < time.Duration(graceSec)*time.Second
				if unenriched && justSeen {
					deferred++
					continue
				}
				ready = append(ready, l)
			}
			if deferred > 0 {
				searchLog.InfoContext(searchCtx, "deferred unenriched listings for enrich grace window",
					"deferred", deferred, "ready", len(ready), "grace_seconds", graceSec)
			}
			acc.result.newListings = ready
		}

		// Deliver notifications.
		if len(acc.result.newListings) > 0 || len(acc.result.priceDropMessages) > 0 {
			searchLog.InfoContext(searchCtx, "search matched listings, preparing delivery",
				"new", len(acc.result.newListings),
				"price_drops", len(acc.result.priceDropMessages),
			)
		}
		delivered := s.deliverResults(searchCtx, acc.search, acc.lang, acc.result, searchLog)
		if delivered {
			stats.notificationsSent++
		}
	}

	// Durably record staged price observations now that listing outcomes
	// are settled.
	s.flushPendingPrices(ctx, pendingPrices, persistedTokens, persistFailedTokens)

	// Write per-search cycle stats for dashboard observability.
	if s.stores.SearchCycleStats != nil && len(searches) > 0 {
		rejections := s.percolator.CountRejections(raw)
		now := time.Now()

		cycleStats := make([]storage.SearchCycleStats, 0, len(searches))
		for _, search := range searches {
			acc := accums[search.ID]
			ctr := searchCounters[search.ID]
			matched := 0
			kmFiltered := 0
			if ctr != nil {
				matched = ctr.matched
				kmFiltered = ctr.kmFiltered
			}

			rej := rejections[search.ID]

			var newListingCount, priceDropCount int
			var scoreMin, scoreMax, scoreAvg *float64
			var priceMin, priceMax *int
			if acc != nil && len(acc.result.newListings) > 0 {
				newListingCount = len(acc.result.newListings)
				priceDropCount = len(acc.result.priceDropMessages)
				mn, mx, sum := acc.result.newListings[0].FitnessScore, acc.result.newListings[0].FitnessScore, 0.0
				pMin, pMax := acc.result.newListings[0].Price, acc.result.newListings[0].Price
				for _, l := range acc.result.newListings {
					if l.FitnessScore < mn {
						mn = l.FitnessScore
					}
					if l.FitnessScore > mx {
						mx = l.FitnessScore
					}
					sum += l.FitnessScore
					if l.Price > 0 {
						if pMin == 0 || l.Price < pMin {
							pMin = l.Price
						}
						if l.Price > pMax {
							pMax = l.Price
						}
					}
				}
				avg := sum / float64(len(acc.result.newListings))
				scoreMin = &mn
				scoreMax = &mx
				scoreAvg = &avg
				if pMin > 0 {
					priceMin = &pMin
					priceMax = &pMax
				}
			} else if acc != nil {
				priceDropCount = len(acc.result.priceDropMessages)
			}

			cycleStats = append(cycleStats, storage.SearchCycleStats{
				SearchID:    search.ID,
				ChatID:      search.ChatID,
				SearchName:  search.Name,
				CycleAt:     now,
				FeedSize:    matched + rej[percolator.RejectYearOut] + rej[percolator.RejectPriceOut] + rej[percolator.RejectKmOver] + rej[percolator.RejectHandOver] + rej[percolator.RejectEngineCC] + rej[percolator.RejectSeller] + rej[percolator.RejectOtherFilter],
				Matched:     matched,
				NewListings: newListingCount,
				KmFiltered:  kmFiltered,
				Delivered:   newListingCount,
				PriceDrops:  priceDropCount,
				WrongModel:  rej[percolator.RejectWrongModel],
				YearOut:     rej[percolator.RejectYearOut],
				PriceOut:    rej[percolator.RejectPriceOut],
				KmOver:      rej[percolator.RejectKmOver],
				HandOver:    rej[percolator.RejectHandOver],
				EngineCC:    rej[percolator.RejectEngineCC],
				Seller:      rej[percolator.RejectSeller],
				OtherFilter: rej[percolator.RejectOtherFilter],
				ScoreMin:    scoreMin,
				ScoreMax:    scoreMax,
				ScoreAvg:    scoreAvg,
				PriceMin:    priceMin,
				PriceMax:    priceMax,
			})
		}
		if err := s.stores.SearchCycleStats.UpsertSearchCycleStats(ctx, cycleStats); err != nil {
			s.logger.Error("failed to write per-search cycle stats", "error", err)
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

func (s *Scheduler) refreshMarketView(ctx context.Context) {
	if s.stores.Market == nil {
		return
	}
	if err := s.stores.Market.RefreshMarketMedians(ctx); err != nil {
		s.logger.Error("background market view refresh failed", "error", err)
	} else {
		s.logger.Debug("background market view refreshed")
	}
}

func (s *Scheduler) buildMarketCache(ctx context.Context) *scoring.MarketCache {
	refreshStart := time.Now()
	rows, err := s.stores.Market.LoadMarketMedians(ctx)
	if err != nil {
		s.logger.Error("failed to load market medians from database, keeping previous cache", "error", err)
		return nil
	}
	s.logger.Debug("market medians loaded", "rows", len(rows), "duration_ms", time.Since(refreshStart).Milliseconds())

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
func (s *Scheduler) prefillFromDB(ctx context.Context, listings []model.RawListing) bool {
	var tokens []string
	for i := range listings {
		if listings[i].Km <= 0 || listings[i].City == "" || listings[i].ImageURL == "" {
			tokens = append(tokens, listings[i].Token)
		}
	}
	if len(tokens) == 0 {
		return true
	}

	prefillStart := time.Now()
	data, err := s.stores.Listings.LookupEnrichmentData(ctx, tokens)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to look up enrichment data from database for prefill",
			"error", err, "looked_up", len(tokens))
		return false
	}
	if len(data) == 0 {
		return true
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
		s.logger.InfoContext(ctx, "prefilled listing data from database to supplement missing fields",
			"filled", filled, "looked_up", len(tokens),
			"duration_ms", time.Since(prefillStart).Milliseconds())
	}
	return true
}

// deactivateExcessSearches pauses the newest searches beyond maxActive for a
// user whose tier was downgraded, keeping the oldest ones active.
func (s *Scheduler) deactivateExcessSearches(ctx context.Context, chatID int64, maxActive int) {
	if s.stores.Searches == nil {
		return
	}
	searches, err := s.stores.Searches.ListSearches(ctx, chatID)
	if err != nil {
		s.logger.Error("failed to list searches for tier downgrade", "chat_id", chatID, "error", err)
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
			s.logger.Error("failed to deactivate excess search during tier downgrade", "chat_id", chatID, "search_id", active[i].ID, "error", err)
		}
	}
	s.logger.Info("deactivated excess searches after tier downgrade",
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
