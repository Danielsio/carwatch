package yad2

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"
)

// relayClient routes HTTP requests through a Cloudflare Worker relay,
// so Yad2 sees Cloudflare edge IPs instead of the scraper's datacenter IP.
type relayClient struct {
	httpClient *http.Client
	relayURL   string
	secret     string
	userAgents []string
}

// NewRelayClient creates a client that routes all requests through the
// Cloudflare Worker at relayURL. The secret is sent as X-Relay-Secret
// for authentication.
func NewRelayClient(relayURL, secret string, userAgents []string) *relayClient {
	return &relayClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		relayURL:   relayURL,
		secret:     secret,
		userAgents: userAgents,
	}
}

func (c *relayClient) Get(ctx context.Context, targetURL string) (*HTTPResult, error) {
	reqURL := c.relayURL + "?target=" + url.QueryEscape(targetURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("relay: create request: %w", err)
	}

	req.Header.Set("X-Relay-Secret", c.secret)
	if len(c.userAgents) > 0 {
		req.Header.Set("User-Agent", c.userAgents[rand.IntN(len(c.userAgents))])
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "he-IL,he;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("DNT", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("relay: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("relay: read response: %w", err)
	}

	return &HTTPResult{
		Body:       body,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}, nil
}

func (c *relayClient) Close() {}
