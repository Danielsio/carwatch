package fetcher

import (
	"context"
	"errors"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

var (
	ErrChallenge   = errors.New("anti-bot challenge detected")
	ErrRateLimited = errors.New("rate limited")
)

type Fetcher interface {
	Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error)
}
