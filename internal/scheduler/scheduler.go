package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

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
	fetchTimeout     = 60 * time.Second
	maxBackoff       = 4.0
	minBackoff       = 1.0
	pruneInterval    = 24 * time.Hour
	maxRetries       = 3
	retryBaseDelay   = 2 * time.Second
)

type CatalogIngester interface {
	Ingest(ctx context.Context, manufacturerID int, manufacturerName string, modelID int, modelName string)
	Flush(ctx context.Context)
}

type Scheduler struct {
	cfgMu             sync.RWMutex
	cfg               *config.Config
	configPath        string
	fetcher           fetcher.Fetcher
	dedup             storage.DedupStore
	notifier          notifier.Notifier
	logger            *slog.Logger
	loc               *time.Location
	boMu              sync.RWMutex
	backoffMultiplier float64
	lastPruneTime     time.Time
	observer          CycleObserver
	queue             storage.NotificationQueue
	prices            storage.PriceTracker
	fetcherFactory    *fetcher.Factory
	listingStore      storage.ListingStore
	searchStore       storage.SearchStore
	userStore         storage.UserStore
	digestStore       storage.DigestStore
	hiddenStore       storage.HiddenListingStore
	catalogIngester   CatalogIngester
	marketStore       storage.MarketStore
	dailyDigestStore  storage.DailyDigestStore
	triggerCh         chan struct{}

	langCache    sync.Map
	digestCache  sync.Map
	premiumCache sync.Map
}

type digestMeta struct {
	mode     string
	interval string
}

type Options struct {
	Observer        CycleObserver
	Queue           storage.NotificationQueue
	Prices          storage.PriceTracker
	ConfigPath      string
	FetcherFactory  *fetcher.Factory
	ListingStore    storage.ListingStore
	SearchStore     storage.SearchStore
	UserStore       storage.UserStore
	DigestStore     storage.DigestStore
	HiddenStore      storage.HiddenListingStore
	CatalogIngester  CatalogIngester
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
		cfg:               cfg,
		configPath:        opts.ConfigPath,
		fetcher:           f,
		dedup:             d,
		notifier:          n,
		logger:            logger,
		loc:               loc,
		backoffMultiplier: 1.0,
		observer:          obs,
		queue:             opts.Queue,
		prices:            opts.Prices,
		fetcherFactory:    opts.FetcherFactory,
		listingStore:      opts.ListingStore,
		searchStore:       opts.SearchStore,
		userStore:         opts.UserStore,
		digestStore:       opts.DigestStore,
		hiddenStore:       opts.HiddenStore,
		catalogIngester:   opts.CatalogIngester,
		marketStore:       opts.MarketStore,
		dailyDigestStore:  opts.DailyDigestStore,
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
		"interval", logInterval,
		"jitter", logJitter,
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

	for {
		delay := s.nextDelay()

		if !s.isActiveHours() {
			if sleepUntil := s.durationUntilActiveStart(); sleepUntil > 0 {
				s.logger.Info("outside active hours, sleeping until start",
					"sleep", sleepUntil.Round(time.Minute),
				)
				delay = sleepUntil
			}
		}

		s.logger.Info("next poll", "delay", delay.Round(time.Second))

		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopping")
			return ctx.Err()
		case <-sighup:
			s.reloadConfig()
			continue
		case <-s.triggerCh:
			s.logger.Info("poll triggered")
		case <-time.After(delay):
		}

		if !s.isActiveHours() {
			continue
		}

		if err := cycle(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			s.logger.Error("cycle failed", "error", err)
		}
	}
}

func (s *Scheduler) deliveryFor(ctx context.Context, chatID int64, lang locale.Lang) DeliveryStrategy {
	if s.digestStore != nil {
		var mode string
		if v, ok := s.digestCache.Load(chatID); ok {
			mode = v.(digestMeta).mode
		} else {
			m, interval, err := s.digestStore.GetDigestMode(ctx, chatID)
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
			return NewDigestDelivery(s.digestStore, lang)
		}
	}
	return NewInstantDelivery(s.notifier, s.queue, lang)
}

func (s *Scheduler) fetcherForSource(source string) fetcher.Fetcher {
	if s.fetcherFactory != nil {
		if f, ok := s.fetcherFactory.Get(source); ok {
			return f
		}
	}
	return s.fetcher
}

func (s *Scheduler) fetchWithRetryUsing(ctx context.Context, f fetcher.Fetcher, params config.SourceParams) ([]model.RawListing, error) {
	var lastErr error
	for attempt := range maxRetries {
		listings, err := f.Fetch(ctx, params)
		if err == nil {
			return listings, nil
		}
		lastErr = err

		if errors.Is(err, fetcher.ErrPartialResults) && len(listings) > 0 {
			s.logger.Warn("fetch returned partial results", "error", err, "count", len(listings))
			return listings, nil
		}

		if errors.Is(err, fetcher.ErrChallenge) || errors.Is(err, fetcher.ErrCircuitOpen) || errors.Is(err, context.Canceled) {
			return nil, err
		}

		if attempt < maxRetries-1 {
			delay := retryBaseDelay * (1 << attempt)
			s.logger.Warn("fetch failed, retrying",
				"attempt", attempt+1,
				"delay", delay,
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

	return currentMinutes >= start && currentMinutes < end
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
	now := time.Now().In(loc)
	currentMinutes := now.Hour()*60 + now.Minute()

	diffMinutes := startMinutes - currentMinutes
	if diffMinutes <= 0 {
		diffMinutes += 24 * 60
	}
	return time.Duration(diffMinutes) * time.Minute
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
	if s.queue == nil {
		return
	}
	pending, err := s.queue.PendingNotifications(ctx)
	if err != nil {
		s.logger.Error("failed to load pending notifications", "error", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	s.logger.Info("retrying pending notifications", "count", len(pending))
	for _, p := range pending {
		if err := s.notifier.NotifyRaw(ctx, p.Recipient, p.Payload); err != nil {
			s.logger.Error("retry notification failed",
				"recipient", maskPhone(p.Recipient),
				"error", err,
			)
			continue
		}
		if err := s.queue.AckNotification(ctx, p.ID); err != nil {
			s.logger.Error("ack notification failed", "id", p.ID, "error", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Scheduler) runMultiTenantCycle(ctx context.Context) error {
	s.logger.Info("starting poll cycle")

	s.langCache.Range(func(k, _ any) bool { s.langCache.Delete(k); return true })
	s.digestCache.Range(func(k, _ any) bool { s.digestCache.Delete(k); return true })
	s.premiumCache.Range(func(k, _ any) bool { s.premiumCache.Delete(k); return true })

	searches, err := s.searchStore.ListAllActiveSearches(ctx)
	if err != nil {
		return fmt.Errorf("load searches: %w", err)
	}

	s.pruneIfDue(ctx)
	s.processExpiredPremium(ctx)

	if len(searches) == 0 {
		s.logger.Info("no active searches")
		return nil
	}

	var marketCache *scoring.MarketCache
	if s.marketStore != nil {
		listings, err := s.marketStore.MarketListings(ctx)
		if err != nil {
			s.logger.Error("load market data failed", "error", err)
		} else {
			data := make([]scoring.ListingData, len(listings))
			for i, l := range listings {
				data[i] = scoring.ListingData{
					Manufacturer: l.Manufacturer,
					Model:        l.Model,
					Year:         l.Year,
					Price:        l.Price,
				}
			}
			marketCache = scoring.NewMarketCache(data)
		}
	}

	groups := GroupSearches(searches)
	s.logger.Info("grouped searches", "groups", len(groups), "total_searches", len(searches))

	s.cfgMu.RLock()
	concurrency := s.cfg.Polling.MaxConcurrentFetches
	s.cfgMu.RUnlock()
	if concurrency <= 0 {
		concurrency = 4
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allFailed := true

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
					s.logger.Error("panic in processGroup",
						"manufacturer", g.Manufacturer,
						"model", g.Model,
						"panic", r,
					)
					s.observer.RecordError()
				}
			}()

			if err := s.processGroup(ctx, g, marketCache); err != nil {
				s.logger.Error("group failed",
					"manufacturer", g.Manufacturer,
					"model", g.Model,
					"error", err,
				)
				if errors.Is(err, fetcher.ErrChallenge) {
					s.boMu.Lock()
					s.backoffMultiplier = min(s.backoffMultiplier*2, maxBackoff)
					s.boMu.Unlock()
				}
				return
			}
			mu.Lock()
			allFailed = false
			mu.Unlock()
			s.boMu.Lock()
			s.backoffMultiplier = max(s.backoffMultiplier/2, minBackoff)
			s.boMu.Unlock()
		}(group)
	}
	wg.Wait()

	if s.catalogIngester != nil {
		s.catalogIngester.Flush(ctx)
	}

	s.processDigests(ctx)
	s.processDailyDigests(ctx)

	if allFailed && len(groups) > 0 {
		s.observer.RecordError()
		return fmt.Errorf("all %d groups failed", len(groups))
	}

	s.observer.RecordSuccess()
	return nil
}

func (s *Scheduler) pruneIfDue(ctx context.Context) {
	if time.Since(s.lastPruneTime) <= pruneInterval {
		return
	}
	s.cfgMu.RLock()
	pruneAfter := s.cfg.Storage.PruneAfter
	s.cfgMu.RUnlock()
	if pruneAfter > 0 {
		pruned, err := s.dedup.Prune(ctx, pruneAfter)
		if err != nil {
			s.logger.Error("prune failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listings", "count", pruned)
		}
	}
	if s.queue != nil {
		pruned, err := s.queue.PruneNotifications(ctx, 48*time.Hour)
		if err != nil {
			s.logger.Error("prune notifications failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned expired notifications", "count", pruned)
		}
	}
	if s.prices != nil {
		pruned, err := s.prices.PrunePrices(ctx, 90*24*time.Hour)
		if err != nil {
			s.logger.Error("prune prices failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old price history", "count", pruned)
		}
	}
	s.lastPruneTime = time.Now()
}

func (s *Scheduler) processGroup(ctx context.Context, group CanonicalGroup, marketCache *scoring.MarketCache) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	source := group.Source
	if source == "" {
		source = "yad2"
	}
	activeFetcher := s.fetcherForSource(source)
	fetchStart := time.Now()
	raw, err := s.fetchWithRetryUsing(fetchCtx, activeFetcher, group.Params)
	s.observer.RecordFetch(source, time.Since(fetchStart), err)
	if err != nil {
		return err
	}

	if s.catalogIngester != nil {
		for _, l := range raw {
			s.catalogIngester.Ingest(ctx, l.ManufacturerID, l.Manufacturer, l.ModelID, l.Model)
		}
	}

	s.logger.Info("fetched for group",
		"manufacturer", group.Manufacturer,
		"model", group.Model,
		"raw_count", len(raw),
		"user_searches", len(group.Searches),
	)

	for _, search := range group.Searches {
		criteria := config.FilterCriteria{
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

		filtered := filter.Apply(criteria, raw)

		lang := s.userLang(ctx, search.ChatID)
		isPremium := s.isUserPremium(ctx, search.ChatID)

		var hiddenTokens map[string]bool
		if s.hiddenStore != nil {
			var err error
			hiddenTokens, err = s.hiddenStore.ListHiddenTokens(ctx, search.ChatID)
			if err != nil {
				s.logger.Error("load hidden tokens failed", "chat_id", search.ChatID, "error", err)
			}
		}

		var newListings []model.Listing
		var priceDropMessages []string
		var listingRecords []storage.ListingRecord
		for _, l := range filtered {
			if hiddenTokens[l.Token] {
				continue
			}

			isNew, err := s.dedup.ClaimNew(ctx, l.Token, search.ChatID, search.ID)
			if err != nil {
				s.logger.Error("claim failed", "token", l.Token, "error", err)
				continue
			}

			if s.prices != nil && l.Price > 0 {
				oldPrice, changed, err := s.prices.RecordPrice(ctx, l.Token, l.Price)
				if err != nil {
					s.logger.Error("record price failed", "token", l.Token, "error", err)
				} else if changed && l.Price < oldPrice && isPremium {
					s.logger.Info("price drop detected",
						"token", l.Token,
						"old_price", oldPrice,
						"new_price", l.Price,
					)
					listing := model.Listing{RawListing: l, SearchName: search.Name}
					priceDropMessages = append(priceDropMessages, notifier.FormatPriceDrop(listing, oldPrice, lang))
					if s.listingStore != nil {
						_ = s.listingStore.SaveListing(ctx, storage.ListingRecord{
							Token: l.Token, ChatID: search.ChatID, SearchName: search.Name,
							Manufacturer: l.Manufacturer, Model: l.Model,
							Year: l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
							City: l.City, PageLink: l.PageLink, FirstSeenAt: time.Now(),
						})
					}
					continue
				}
			}

			if !isNew {
				continue
			}

			listing := model.Listing{RawListing: l, SearchName: search.Name}
			listing.FitnessScore = scoring.FitnessScore(scoring.FitnessParams{
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
			})
			if marketCache != nil && l.Price > 0 && isPremium {
				median, cohort, ok := marketCache.Lookup(l.Manufacturer, l.Model, l.Year)
				if ok {
					listing.DealScore = &model.ScoreInfo{
						Score:       scoring.Score(l.Price, median),
						MedianPrice: median,
						CohortSize:  cohort,
					}
				}
			}
			newListings = append(newListings, listing)

			listingRecords = append(listingRecords, storage.ListingRecord{
				Token: l.Token, ChatID: search.ChatID, SearchName: search.Name,
				Manufacturer: l.Manufacturer, Model: l.Model,
				Year: l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
				City: l.City, PageLink: l.PageLink, FirstSeenAt: time.Now(),
			})
		}

		if s.listingStore != nil && len(listingRecords) > 0 {
			if err := s.listingStore.SaveListings(ctx, listingRecords); err != nil {
				s.logger.Error("batch save listings failed", "error", err)
				for _, rec := range listingRecords {
					if relErr := s.dedup.ReleaseClaim(ctx, rec.Token, rec.ChatID); relErr != nil {
						s.logger.Error("release claim after batch save failure",
							"token", rec.Token, "chat_id", rec.ChatID, "error", relErr)
					}
				}
				continue
			}
		}

		delivery := s.deliveryFor(ctx, search.ChatID, lang)

		for _, msg := range priceDropMessages {
			if err := delivery.DeliverRaw(ctx, search.ChatID, msg); err != nil {
				if errors.Is(err, notifier.ErrRecipientBlocked) {
					s.logger.Warn("user blocked bot, deactivating",
						"chat_id", search.ChatID,
					)
					if s.userStore != nil {
						_ = s.userStore.SetUserActive(context.Background(), search.ChatID, false)
					}
					break
				}
				s.logger.Error("price drop delivery failed",
					"chat_id", search.ChatID,
					"error", err,
				)
			}
		}

		if len(newListings) == 0 {
			continue
		}

		s.observer.RecordListingsFound(len(newListings))

		s.logger.Info("new listings for user",
			"chat_id", search.ChatID,
			"search", search.Name,
			"count", len(newListings),
		)

		if err := delivery.DeliverBatch(ctx, search.ChatID, newListings); err != nil {
			if errors.Is(err, notifier.ErrRecipientBlocked) {
				s.logger.Warn("user blocked bot, deactivating",
					"chat_id", search.ChatID,
				)
				if s.userStore != nil {
					_ = s.userStore.SetUserActive(context.Background(), search.ChatID, false)
				}
			} else {
				s.logger.Error("delivery failed",
					"chat_id", search.ChatID,
					"error", err,
				)
			}
			for _, l := range newListings {
				_ = s.dedup.ReleaseClaim(context.Background(), l.Token, search.ChatID)
			}
		} else {
			s.observer.RecordNotificationSent()
		}
	}

	return nil
}

func (s *Scheduler) processDigests(ctx context.Context) {
	if s.digestStore == nil {
		return
	}

	users, err := s.digestStore.PendingDigestUsers(ctx)
	if err != nil {
		s.logger.Error("list pending digest users failed", "error", err)
		return
	}

	for _, chatID := range users {
		mode, intervalStr, err := s.digestStore.GetDigestMode(ctx, chatID)
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

		lastFlushed, err := s.digestStore.DigestLastFlushed(ctx, chatID)
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
	payloads, cutoff, err := s.digestStore.PeekDigest(ctx, chatID)
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

	if err := s.digestStore.AckDigest(ctx, chatID, cutoff); err != nil {
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
	if s.dailyDigestStore == nil {
		return
	}

	users, err := s.dailyDigestStore.ListDailyDigestUsers(ctx)
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
	stats, err := s.dailyDigestStore.DailyStats(ctx, chatID)
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

	if err := s.dailyDigestStore.UpdateDailyDigestLastSent(ctx, chatID); err != nil {
		s.logger.Error("daily digest last-sent update failed after successful send, digest may be resent",
			"chat_id", chatID,
			"error", err,
		)
	}

	s.logger.Info("daily digest sent", "chat_id", chatID, "searches", len(stats))
}

func (s *Scheduler) deactivateExcessSearches(ctx context.Context, chatID int64, maxActive int) {
	if s.searchStore == nil {
		return
	}
	searches, err := s.searchStore.ListSearches(ctx, chatID)
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
		if err := s.searchStore.SetSearchActive(ctx, active[i].ID, chatID, false); err != nil {
			s.logger.Error("deactivate excess search failed", "chat_id", chatID, "search_id", active[i].ID, "error", err)
		}
	}
	s.logger.Info("deactivated excess searches on downgrade",
		"chat_id", chatID, "paused", len(active)-maxActive, "kept", maxActive)
}

func (s *Scheduler) isUserPremium(ctx context.Context, chatID int64) bool {
	if v, ok := s.premiumCache.Load(chatID); ok {
		return v.(bool)
	}
	premium := false
	if s.userStore != nil {
		user, err := s.userStore.GetUser(ctx, chatID)
		if err != nil {
			s.logger.Error("premium check failed, defaulting to free", "chat_id", chatID, "error", err)
			return false
		} else if user != nil {
			premium = user.Tier == "premium" && (user.TierExpires.IsZero() || user.TierExpires.After(time.Now()))
		}
	}
	s.premiumCache.Store(chatID, premium)
	return premium
}

func (s *Scheduler) processExpiredPremium(ctx context.Context) {
	if s.userStore == nil {
		return
	}
	expired, err := s.userStore.ListExpiredPremium(ctx)
	if err != nil {
		s.logger.Error("list expired premium failed", "error", err)
		return
	}
	for _, u := range expired {
		if err := s.userStore.SetUserTier(ctx, u.ChatID, "free", time.Time{}); err != nil {
			s.logger.Error("downgrade user failed", "chat_id", u.ChatID, "error", err)
			continue
		}
		if s.dailyDigestStore != nil {
			if err := s.dailyDigestStore.SetDailyDigest(ctx, u.ChatID, false, "09:00"); err != nil {
				s.logger.Error("disable daily digest on downgrade failed", "chat_id", u.ChatID, "error", err)
			}
		}
		s.deactivateExcessSearches(ctx, u.ChatID, 1)
		lang := s.userLang(ctx, u.ChatID)
		chatIDStr := fmt.Sprintf("%d", u.ChatID)
		if err := s.notifier.NotifyRaw(ctx, chatIDStr, locale.T(lang, "premium_expired")); err != nil {
			s.logger.Error("send expiry notification failed", "chat_id", u.ChatID, "error", err)
		}
		s.logger.Info("premium expired, downgraded to free", "chat_id", u.ChatID)
	}
}

func (s *Scheduler) userLang(ctx context.Context, chatID int64) locale.Lang {
	if v, ok := s.langCache.Load(chatID); ok {
		return v.(locale.Lang)
	}
	lang := locale.Hebrew
	if s.userStore != nil {
		user, err := s.userStore.GetUser(ctx, chatID)
		if err == nil && user != nil && user.Language != "" {
			lang = locale.Lang(user.Language)
		}
	}
	s.langCache.Store(chatID, lang)
	return lang
}

func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}
