package pricelist

import (
	"context"

	"github.com/dsionov/carwatch/internal/fetcher/yad2"
)

// yad2Adapter wraps a yad2.HTTPDoer to satisfy pricelist.HTTPDoer.
type yad2Adapter struct {
	inner yad2.HTTPDoer
}

// NewYad2Client wraps a yad2.HTTPDoer (azuretls stealth client) for use
// by the pricelist fetcher, reusing the same TLS fingerprint and proxy
// configuration that bypasses Yad2's anti-bot protection.
func NewYad2Client(client yad2.HTTPDoer) HTTPDoer {
	return &yad2Adapter{inner: client}
}

func (a *yad2Adapter) Get(ctx context.Context, url string) (*HTTPResult, error) {
	res, err := a.inner.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	return &HTTPResult{
		Body:       res.Body,
		StatusCode: res.StatusCode,
	}, nil
}
