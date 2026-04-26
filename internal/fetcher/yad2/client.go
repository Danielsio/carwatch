package yad2

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	azuretls "github.com/Noooste/azuretls-client"
)

// HTTPResult holds the outcome of an HTTP GET request.
type HTTPResult struct {
	Body       []byte
	StatusCode int
	Header     http.Header
}

// HTTPDoer abstracts HTTP GET requests so the production azuretls client
// can be swapped for a plain net/http client in tests.
type HTTPDoer interface {
	Get(ctx context.Context, url string) (*HTTPResult, error)
	Close()
}

// --- Production client (azuretls with Chrome TLS fingerprint) ---

type stealthClient struct {
	session    *azuretls.Session
	userAgents []string
}

func newStealthClient(userAgents []string, proxy string) (*stealthClient, error) {
	session := azuretls.NewSession()
	session.SetTimeout(30 * time.Second)

	if proxy != "" {
		if err := session.SetProxy(proxy); err != nil {
			session.Close()
			return nil, fmt.Errorf("set proxy: %w", err)
		}
	}

	return &stealthClient{session: session, userAgents: userAgents}, nil
}

func (c *stealthClient) Get(ctx context.Context, reqURL string) (*HTTPResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ua := c.userAgents[rand.IntN(len(c.userAgents))]

	type fetchResult struct {
		resp *azuretls.Response
		err  error
	}
	ch := make(chan fetchResult, 1)
	go func() {
		r, e := c.session.Get(reqURL, azuretls.OrderedHeaders{
			{"User-Agent", ua},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
			{"Accept-Language", "he-IL,he;q=0.9,en-US;q=0.8,en;q=0.7"},
			{"Accept-Encoding", "gzip, deflate, br"},
			{"DNT", "1"},
			{"Upgrade-Insecure-Requests", "1"},
			{"Sec-Fetch-Dest", "document"},
			{"Sec-Fetch-Mode", "navigate"},
			{"Sec-Fetch-Site", "none"},
			{"Sec-Fetch-User", "?1"},
			{"Cache-Control", "max-age=0"},
		})
		ch <- fetchResult{r, e}
	}()

	var resp *azuretls.Response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		resp = result.resp
	}

	header := make(http.Header, len(resp.Header))
	for k, v := range resp.Header {
		header[k] = v
	}

	return &HTTPResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
		Header:     header,
	}, nil
}

func (c *stealthClient) Close() {
	c.session.Close()
}

// --- Plain client for tests (uses net/http) ---

type plainClient struct {
	httpClient *http.Client
	userAgents []string
}

func newPlainClient(userAgents []string, proxy string) (*plainClient, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 10
	transport.MaxIdleConnsPerHost = 5
	transport.IdleConnTimeout = 90 * time.Second

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &plainClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		userAgents: userAgents,
	}, nil
}

func (c *plainClient) Get(ctx context.Context, reqURL string) (*HTTPResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	ua := c.userAgents[rand.IntN(len(c.userAgents))]
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "he-IL,he;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("DNT", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	return &HTTPResult{
		Body:       body,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}, nil
}

func (c *plainClient) Close() {}

// NewClient creates the production stealth client with Chrome TLS fingerprint.
func NewClient(userAgents []string, proxy string) (HTTPDoer, error) {
	if len(userAgents) == 0 {
		return nil, fmt.Errorf("at least one user agent is required")
	}
	return newStealthClient(userAgents, proxy)
}

// NewPlainClient creates a plain net/http client (for tests against httptest servers).
func NewPlainClient(userAgents []string, proxy string) (HTTPDoer, error) {
	if len(userAgents) == 0 {
		return nil, fmt.Errorf("at least one user agent is required")
	}
	return newPlainClient(userAgents, proxy)
}

func redactProxy(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid>"
	}
	u.User = nil
	return u.String()
}
