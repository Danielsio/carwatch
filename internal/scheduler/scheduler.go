package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/fetcher"
	"github.com/dsionov/carwatch/internal/filter"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
	"github.com/dsionov/carwatch/internal/storage"
)

type Scheduler struct {
	cfg      *config.Config
	fetcher  fetcher.Fetcher
	dedup    storage.DedupStore
	notifier notifier.Notifier
	logger   *slog.Logger
	loc      *time.Location
}

func New(
	cfg *config.Config,
	f fetcher.Fetcher,
	d storage.DedupStore,
	n notifier.Notifier,
	logger *slog.Logger,
) (*Scheduler, error) {
	loc, err := time.LoadLocation(cfg.Polling.Timezone)
	if err != nil {
		return nil, err
	}
	return &Scheduler{
		cfg:      cfg,
		fetcher:  f,
		dedup:    d,
		notifier: n,
		logger:   logger,
		loc:      loc,
	}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info("scheduler started",
		"interval", s.cfg.Polling.Interval,
		"jitter", s.cfg.Polling.Jitter,
		"searches", len(s.cfg.Searches),
	)

	if err := s.runCycle(ctx); err != nil {
		s.logger.Error("initial cycle failed", "error", err)
	}

	for {
		delay := s.nextDelay()
		s.logger.Info("next poll in", "delay", delay.Round(time.Second))

		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopping")
			return ctx.Err()
		case <-time.After(delay):
		}

		if !s.isActiveHours() {
			s.logger.Info("outside active hours, skipping")
			continue
		}

		if err := s.runCycle(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			s.logger.Error("cycle failed", "error", err)
		}
	}
}

func (s *Scheduler) runCycle(ctx context.Context) error {
	s.logger.Info("starting poll cycle")

	for _, search := range s.cfg.Searches {
		if err := s.processSearch(ctx, search); err != nil {
			s.logger.Error("search failed",
				"search", search.Name,
				"error", err,
			)
			continue
		}
	}

	if s.cfg.Storage.PruneAfter > 0 {
		pruned, err := s.dedup.Prune(ctx, s.cfg.Storage.PruneAfter)
		if err != nil {
			s.logger.Error("prune failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("pruned old listings", "count", pruned)
		}
	}

	return nil
}

func (s *Scheduler) processSearch(ctx context.Context, search config.SearchConfig) error {
	raw, err := s.fetcher.Fetch(ctx, search.Params)
	if err != nil {
		if errors.Is(err, fetcher.ErrChallenge) {
			s.logger.Warn("anti-bot challenge detected, backing off")
		}
		return err
	}

	filtered := filter.Apply(search.Filters, raw)
	s.logger.Info("filtered listings",
		"search", search.Name,
		"total", len(raw),
		"after_filter", len(filtered),
	)

	var newListings []model.Listing
	for _, l := range filtered {
		isNew, err := s.dedup.ClaimNew(ctx, l.Token, search.Name)
		if err != nil {
			return err
		}
		if !isNew {
			continue
		}
		newListings = append(newListings, model.Listing{
			RawListing: l,
			SearchName: search.Name,
		})
	}

	s.logger.Info("new listings found",
		"search", search.Name,
		"count", len(newListings),
	)

	if len(newListings) == 0 {
		return nil
	}

	anyDelivered := false
	for _, recipient := range search.Recipients {
		if err := s.notifier.Notify(ctx, recipient, newListings); err != nil {
			s.logger.Error("notification failed",
				"recipient", recipient,
				"error", err,
			)
			continue
		}
		anyDelivered = true
	}

	if !anyDelivered {
		s.logger.Warn("all recipients failed, releasing claims for retry",
			"search", search.Name,
			"count", len(newListings),
		)
		for _, l := range newListings {
			if err := s.dedup.ReleaseClaim(ctx, l.Token); err != nil {
				s.logger.Error("release claim failed", "token", l.Token, "error", err)
			}
		}
	}

	return nil
}

func (s *Scheduler) nextDelay() time.Duration {
	base := s.cfg.Polling.Interval
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

	start, err := parseTimeOfDay(ah.Start)
	if err != nil {
		return true
	}
	end, err := parseTimeOfDay(ah.End)
	if err != nil {
		return true
	}

	return currentMinutes >= start && currentMinutes <= end
}

func parseTimeOfDay(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}
