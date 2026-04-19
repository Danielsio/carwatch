package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/health"
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
	maxPages         = 3
)

type Scheduler struct {
	cfg               *config.Config
	configPath        string
	fetcher           fetcher.Fetcher
	dedup             storage.DedupStore
	notifier          notifier.Notifier
	logger            *slog.Logger
	loc               *time.Location
	backoffMultiplier float64
	lastPruneTime     time.Time
	health            *health.Status
	queue             storage.NotificationQueue
	prices            storage.PriceTracker
	fetcherFactory    *fetcher.Factory
	listingStore      storage.ListingStore
	searchStore       storage.SearchStore
}

type Options struct {
	Health         *health.Status
	Queue          storage.NotificationQueue
	Prices         storage.PriceTracker
	ConfigPath     string
	FetcherFactory *fetcher.Factory
	ListingStore   storage.ListingStore
	SearchStore    storage.SearchStore
}

func New(
	cfg *config.Config,
	f fetcher.Fetcher,
	d storage.DedupStore,
	n notifier.Notifier,
	logger *slog.Logger,
	h *health.Status,
) (*Scheduler, error) {
	return NewWithOptions(cfg, f, d, n, logger, Options{Health: h})
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
	return &Scheduler{
		cfg:               cfg,
		configPath:        opts.ConfigPath,
		fetcher:           f,
		dedup:             d,
		notifier:          n,
		logger:            logger,
		loc:               loc,
		backoffMultiplier: 1.0,
		health:            opts.Health,
		queue:             opts.Queue,
		prices:            opts.Prices,
		fetcherFactory:    opts.FetcherFactory,
		listingStore:      opts.ListingStore,
		searchStore:       opts.SearchStore,
	}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info("scheduler started",
		"interval", s.cfg.Polling.Interval,
		"jitter", s.cfg.Polling.Jitter,
		"searches", len(s.cfg.Searches),
	)

	s.retryPending(ctx)

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)

	cycle := s.runCycle
	if s.searchStore != nil {
		cycle = s.runMultiTenantCycle
	}

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

func (s *Scheduler) runCycle(ctx context.Context) error {
	s.logger.Info("starting poll cycle")

	allFailed := true
	for _, search := range s.cfg.Searches {
		if err := s.processSearch(ctx, search); err != nil {
			s.logger.Error("search failed",
				"search", search.Name,
				"error", err,
			)
			if errors.Is(err, fetcher.ErrChallenge) {
				s.backoffMultiplier = min(s.backoffMultiplier*2, maxBackoff)
				s.logger.Warn("increased backoff", "multiplier", s.backoffMultiplier)
			}
			continue
		}
		allFailed = false
		s.backoffMultiplier = max(s.backoffMultiplier/2, minBackoff)
	}

	if time.Since(s.lastPruneTime) > pruneInterval {
		if s.cfg.Storage.PruneAfter > 0 {
			pruned, err := s.dedup.Prune(ctx, s.cfg.Storage.PruneAfter)
			if err != nil {
				s.logger.Error("prune failed", "error", err)
			} else if pruned > 0 {
				s.logger.Info("pruned old listings", "count", pruned)
			}
		}
		s.lastPruneTime = time.Now()
	}

	if allFailed && len(s.cfg.Searches) > 0 {
		if s.health != nil {
			s.health.RecordError()
		}
		return fmt.Errorf("all %d searches failed", len(s.cfg.Searches))
	}

	if s.health != nil {
		s.health.RecordSuccess()
	}
	return nil
}

func (s *Scheduler) fetcherForSource(source string) fetcher.Fetcher {
	if s.fetcherFactory != nil {
		if f, ok := s.fetcherFactory.Get(source); ok {
			return f
		}
	}
	return s.fetcher
}

func (s *Scheduler) processSearch(ctx context.Context, search config.SearchConfig) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	activeFetcher := s.fetcherForSource(search.Source)

	var allRaw []model.RawListing
	for page := range maxPages {
		params := search.Params
		params.Page = page
		raw, err := s.fetchWithRetryUsing(fetchCtx, activeFetcher, params)
		if err != nil {
			if page == 0 {
				return err
			}
			break
		}
		if len(raw) == 0 {
			break
		}
		allRaw = append(allRaw, raw...)

		allNew := true
		for _, l := range raw {
			isNew, _ := s.dedup.ClaimNew(ctx, l.Token, 0, 0)
			if !isNew {
				allNew = false
			}
			if isNew {
				_ = s.dedup.ReleaseClaim(ctx, l.Token, 0)
			}
		}
		if !allNew {
			break
		}
		s.logger.Info("all listings new on page, fetching next", "page", page, "search", search.Name)
	}

	filtered := filter.Apply(search.Filters, allRaw)
	s.logger.Info("filtered listings",
		"search", search.Name,
		"total", len(allRaw),
		"after_filter", len(filtered),
	)

	var newListings []model.Listing
	var priceDropMessages []string
	for _, l := range filtered {
		if s.prices != nil && l.Price > 0 {
			oldPrice, changed, err := s.prices.RecordPrice(ctx, l.Token, l.Price)
			if err != nil {
				s.logger.Error("record price failed", "token", l.Token, "error", err)
			} else if changed {
				s.logger.Info("price drop detected",
					"token", l.Token,
					"old_price", oldPrice,
					"new_price", l.Price,
					"search", search.Name,
				)
				listing := model.Listing{RawListing: l, SearchName: search.Name}
				priceDropMessages = append(priceDropMessages, notifier.FormatPriceDrop(listing, oldPrice))
				continue
			}
		}

		isNew, err := s.dedup.ClaimNew(ctx, l.Token, 0, 0)
		if err != nil {
			return err
		}
		if !isNew {
			continue
		}
		listing := model.Listing{
			RawListing: l,
			SearchName: search.Name,
		}
		newListings = append(newListings, listing)

		if s.listingStore != nil {
			_ = s.listingStore.SaveListing(ctx, storage.ListingRecord{
				Token:        l.Token,
				SearchName:   search.Name,
				Manufacturer: l.Manufacturer,
				Model:        l.Model,
				Year:         l.Year,
				Price:        l.Price,
				Km:           l.Km,
				Hand:         l.Hand,
				City:         l.City,
				PageLink:     l.PageLink,
				FirstSeenAt:  time.Now(),
			})
		}
	}

	for _, msg := range priceDropMessages {
		for _, recipient := range search.Recipients {
			if err := s.notifier.NotifyRaw(ctx, recipient, msg); err != nil {
				s.logger.Error("price drop notification failed",
					"recipient", maskPhone(recipient),
					"error", err,
				)
			}
		}
	}

	s.logger.Info("new listings found",
		"search", search.Name,
		"count", len(newListings),
	)

	if len(newListings) == 0 {
		return nil
	}

	if s.health != nil {
		s.health.RecordListingsFound(len(newListings))
	}

	msg := notifier.FormatBatch(newListings)

	anyDelivered := false
	for _, recipient := range search.Recipients {
		if s.queue != nil {
			if err := s.queue.EnqueueNotification(ctx, recipient, search.Name, msg); err != nil {
				s.logger.Error("enqueue notification failed", "error", err)
			}
		}

		if err := s.notifier.Notify(ctx, recipient, newListings); err != nil {
			s.logger.Error("notification failed",
				"recipient", maskPhone(recipient),
				"error", err,
			)
			continue
		}
		anyDelivered = true
		if s.health != nil {
			s.health.RecordNotificationSent()
		}
	}

	if s.queue != nil {
		s.ackDelivered(ctx, search.Name)
	}

	if !anyDelivered {
		s.logger.Warn("all recipients failed, releasing claims for retry",
			"search", search.Name,
			"count", len(newListings),
		)
		for _, l := range newListings {
			if err := s.dedup.ReleaseClaim(ctx, l.Token, 0); err != nil {
				s.logger.Error("release claim failed", "token", l.Token, "error", err)
			}
		}
	}

	return nil
}

func (s *Scheduler) fetchWithRetry(ctx context.Context, params config.SourceParams) ([]model.RawListing, error) {
	return s.fetchWithRetryUsing(ctx, s.fetcher, params)
}

func (s *Scheduler) fetchWithRetryUsing(ctx context.Context, f fetcher.Fetcher, params config.SourceParams) ([]model.RawListing, error) {
	var lastErr error
	for attempt := range maxRetries {
		listings, err := f.Fetch(ctx, params)
		if err == nil {
			return listings, nil
		}
		lastErr = err

		if errors.Is(err, fetcher.ErrChallenge) || errors.Is(err, context.Canceled) {
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
	base := time.Duration(float64(s.cfg.Polling.Interval) * s.backoffMultiplier)
	jitter := s.cfg.Polling.Jitter
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
	ah := s.cfg.Polling.ActiveHours
	if ah == nil {
		return true
	}

	now := time.Now().In(s.loc)
	currentMinutes := now.Hour()*60 + now.Minute()

	start := parseTimeOfDayOrZero(ah.Start)
	end := parseTimeOfDayOrZero(ah.End)

	if start == 0 && end == 0 {
		return true
	}

	return currentMinutes >= start && currentMinutes < end
}

func (s *Scheduler) durationUntilActiveStart() time.Duration {
	ah := s.cfg.Polling.ActiveHours
	if ah == nil {
		return 0
	}

	startMinutes := parseTimeOfDayOrZero(ah.Start)
	now := time.Now().In(s.loc)
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
	s.cfg = newCfg
	if loc, err := time.LoadLocation(newCfg.Polling.Timezone); err == nil {
		s.loc = loc
	}
	s.logger.Info("config reloaded", "searches", len(newCfg.Searches))
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

func (s *Scheduler) ackDelivered(ctx context.Context, searchName string) {
	pending, err := s.queue.PendingNotifications(ctx)
	if err != nil {
		return
	}
	for _, p := range pending {
		if p.SearchName == searchName {
			_ = s.queue.AckNotification(ctx, p.ID)
		}
	}
}

func (s *Scheduler) runMultiTenantCycle(ctx context.Context) error {
	if s.searchStore == nil {
		return s.runCycle(ctx)
	}

	s.logger.Info("starting multi-tenant poll cycle")

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
	for _, group := range groups {
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
		if s.cfg.Storage.PruneAfter > 0 {
			pruned, err := s.dedup.Prune(ctx, s.cfg.Storage.PruneAfter)
			if err != nil {
				s.logger.Error("prune failed", "error", err)
			} else if pruned > 0 {
				s.logger.Info("pruned old listings", "count", pruned)
			}
		}
		s.lastPruneTime = time.Now()
	}

	if allFailed && len(groups) > 0 {
		if s.health != nil {
			s.health.RecordError()
		}
		return fmt.Errorf("all %d groups failed", len(groups))
	}

	if s.health != nil {
		s.health.RecordSuccess()
	}
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

			if s.prices != nil && l.Price > 0 {
				oldPrice, changed, err := s.prices.RecordPrice(ctx, l.Token, l.Price)
				if err != nil {
					s.logger.Error("record price failed", "token", l.Token, "error", err)
				} else if changed {
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

			isNew, err := s.dedup.ClaimNew(ctx, l.Token, search.ChatID, search.ID)
			if err != nil {
				s.logger.Error("claim failed", "token", l.Token, "error", err)
				continue
			}
			if !isNew {
				continue
			}

			listing := model.Listing{RawListing: l, SearchName: search.Name}
			newListings = append(newListings, listing)

			if s.listingStore != nil {
				_ = s.listingStore.SaveListing(ctx, storage.ListingRecord{
					Token: l.Token, SearchName: search.Name,
					Manufacturer: l.Manufacturer, Model: l.Model,
					Year: l.Year, Price: l.Price, Km: l.Km, Hand: l.Hand,
					City: l.City, PageLink: l.PageLink, FirstSeenAt: time.Now(),
				})
			}
		}

		chatIDStr := fmt.Sprintf("%d", search.ChatID)

		for _, msg := range priceDropMessages {
			if err := s.notifier.NotifyRaw(ctx, chatIDStr, msg); err != nil {
				s.logger.Error("price drop notification failed",
					"chat_id", search.ChatID,
					"error", err,
				)
			}
		}

		if len(newListings) == 0 {
			continue
		}

		if s.health != nil {
			s.health.RecordListingsFound(len(newListings))
		}

		s.logger.Info("new listings for user",
			"chat_id", search.ChatID,
			"search", search.Name,
			"count", len(newListings),
		)

		if err := s.notifier.Notify(ctx, chatIDStr, newListings); err != nil {
			s.logger.Error("notification failed",
				"chat_id", search.ChatID,
				"error", err,
			)
			for _, l := range newListings {
				_ = s.dedup.ReleaseClaim(ctx, l.Token, search.ChatID)
			}
		} else if s.health != nil {
			s.health.RecordNotificationSent()
		}
	}

	return nil
}

func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}
