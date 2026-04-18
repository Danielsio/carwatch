package fetcher

import (
	"context"

	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/model"
)

type Fetcher interface {
	Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error)
}
