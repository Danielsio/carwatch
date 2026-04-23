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
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
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
	backoffMultiplier float64
	lastPruneTime     time.Time
	observer          CycleObserver
	queue             storage.NotificationQueue
	prices            storage.PriceTracker
	fetcherFactory    *fetcher.Factory
	listingStore      storage.ListingStore
	searchStore       storage.SearchStore
	digestStore       storage.DigestStore
	catalogIngester   CatalogIngester
	triggerCh         chan struct{}
}

type Options struct {
	Observer        CycleObserver
	Queue           storage.NotificationQueue
	Prices          storage.PriceTracker
	ConfigPath      string
	FetcherFactory  *fetcher.Factory
	ListingStore    storage.ListingStore
	SearchStore     storage.SearchStore
	DigestStore     storage.DigestStore
	CatalogIngester CatalogIngester
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
		digestStore:       opts.DigestStore,
		catalogIngester:   opts.CatalogIngester,
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

func (s *Scheduler) deliveryFor(ctx context.Context, chatID int64) DeliveryStrategy {
	if s.digestStore != nil {
		mode, _, err := s.digestStore.GetDigestMode(ctx, chatID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				s.logger.Error("get digest mode failed", "chat_id", chatID, "error", err)
			}
		} else if mode == "digest" {
			return NewDigestDelivery(s.digestStore)
		}
	}
	return NewInstantDelivery(s.notifier, s.queue)
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
	base := time.Duration(float64(interval) * s.backoffMultiplier)
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
	}
}

func (s *Scheduler) runMultiTenantCycle(ctx context.Context) error {
	s.logger.Info("starting poll cycle")

	searches, err := s.searchStore.ListAllActiveSearches(ctx)
	if err != nil {
		return fmt.Errorf("load searches: %w", err)
	}

	if len(searches) == 0 {
		s.logger.Info("no active searches")
		return nil
	}

	groups := GroupSearches(searches)
	s.logger.Info("grouped searches", "groups", len(groups), "total_searches", len(searches))

	allFailed := true
	for i, group := range groups {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2*time.Second + time.Duration(rand.Int64N(int64(3*time.Second)))):
			}
		}
		if err := s.processGroup(ctx, group); err != nil {
			s.logger.Error("group failed",
				"manufacturer", group.Manufacturer,
				"model", group.Model,
				"error", err,
			)
			if errors.Is(err, fetcher.ErrChallenge) {
				s.backoffMultiplier = min(s.backoffMultiplier*2, maxBackoff)
			}
			continue
		}
		allFailed = false
		s.backoffMultiplier = max(s.backoffMultiplier/2, minBackoff)
	}

	if time.Since(s.lastPruneTime) > pruneInterval {
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
		s.lastPruneTime = time.Now()
	}

	if s.catalogIngester != nil {
		s.catalogIngester.Flush(ctx)
	}

	s.processDigests(ctx)

	if allFailed && len(groups) > 0 {
		s.observer.RecordError()
		return fmt.Errorf("all %d groups failed", len(groups))
	}

	s.observer.RecordSuccess()
	return nil
}

func (s *Scheduler) processGroup(ctx context.Context, group CanonicalGroup) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	source := group.Source
	if source == "" {
		source = "yad2"
	}
	activeFetcher := s.fetcherForSource(source)
	raw, err := s.fetchWithRetryUsing(fetchCtx, activeFetcher, group.Params)
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
			EngineMinCC: float64(search.EngineMinCC),
			MaxKm:       search.MaxKm,
			MaxHand:     search.MaxHand,
		}

		filtered := filter.Apply(criteria, raw)

		var newListings []model.Listing
		var priceDropMessages []string
		for _, l := range filtered {
			if l.Price > search.PriceMax && search.PriceMax > 0 {
				continue
			}
			if l.Year < search.YearMin || (l.Year > search.YearMax && search.YearMax > 0) {
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
				} else if changed && l.Price < oldPrice {
					s.logger.Info("price drop detected",
						"token", l.Token,
						"old_price", oldPrice,
						"new_price", l.Price,
					)
					listing := model.Listing{RawListing: l, SearchName: search.Name}
					priceDropMessages = append(priceDropMessages, notifier.FormatPriceDrop(listing, oldPrice))
					continue
				}
			}

			if !isNew {
				continue
			}

			listing := model.Listing{RawListing: l, SearchName: search.Name}
			newListings = append(newListings, listing)

			if s.listingStore != nil {
				_ = s.listingStore.SaveListing(ctx, storage.ListingRecord{
					Token: l.Token, ChatID: search.ChatID, SearchName: search.Name,
					Manufacturer: l.Manufacturer, Model: l.Model,
					Year: l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
					City: l.City, PageLink: l.PageLink, FirstSeenAt: time.Now(),
				})
			}
		}

		delivery := s.deliveryFor(ctx, search.ChatID)

		for _, msg := range priceDropMessages {
			if err := delivery.DeliverRaw(ctx, search.ChatID, msg); err != nil {
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
			s.logger.Error("delivery failed",
				"chat_id", search.ChatID,
				"error", err,
			)
			for _, l := range newListings {
				_ = s.dedup.ReleaseClaim(ctx, l.Token, search.ChatID)
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
	payloads, err := s.digestStore.FlushDigest(ctx, chatID)
	if err != nil {
		s.logger.Error("flush digest failed", "chat_id", chatID, "error", err)
		return
	}
	if len(payloads) == 0 {
		return
	}

	chatIDStr := fmt.Sprintf("%d", chatID)
	header := fmt.Sprintf("*Digest Summary (%d items):*\n", len(payloads))
	combined := header + strings.Join(payloads, "\n\n━━━━━━━━━━━━━━━━━━━━\n\n")

	if err := s.notifier.NotifyRaw(ctx, chatIDStr, combined); err != nil {
		s.logger.Error("send digest failed",
			"chat_id", chatID,
			"items", len(payloads),
			"error", err,
		)
		return
	}

	s.logger.Info("digest sent",
		"chat_id", chatID,
		"items", len(payloads),
	)
	s.observer.RecordNotificationSent()
}

func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}
