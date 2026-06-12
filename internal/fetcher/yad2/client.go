package yad2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	azuretls "github.com/Noooste/azuretls-client"
)

// outstandingGoroutines tracks orphaned azuretls goroutines that outlived their context.
var outstandingGoroutines atomic.Int64

// maxInFlightStealthFetches bounds the number of concurrent upstream fetches the
// stealth client will have in flight at once. azuretls has no native context
// cancellation, so a goroutine started for a request keeps running (holding a
// connection) until the request actually completes, even after the caller's
// context is cancelled. Capping in-flight requests means stranded goroutines
// can accumulate to at most this many before new fetches are refused, turning an
// unbounded leak into bounded backpressure. The cap is set well above the
// scheduler's normal fetch concurrency (max_concurrent_fetches defaults to 4) so
// healthy operation never hits it.
const maxInFlightStealthFetches = 16

// errTooManyInFlight is returned when the in-flight fetch limit is reached. It is
// a fail-fast signal: the upstream is hanging and we shed load rather than strand
// another goroutine.
var errTooManyInFlight = errors.New("yad2: too many in-flight fetches, refusing to spawn another")

// inFlightLimiter bounds concurrent upstream fetches whose goroutines may outlive
// a cancelled context. A slot is held until the real request returns, so an
// orphaned (post-cancellation) request still counts against the limit.
type inFlightLimiter struct {
	sem chan struct{}
}

func newInFlightLimiter(max int) *inFlightLimiter {
	if max < 1 {
		max = 1
	}
	return &inFlightLimiter{sem: make(chan struct{}, max)}
}

// run executes fetch in a goroutine and returns its result, or the context error
// if ctx is cancelled first. The goroutine — and the slot it holds — live until
// fetch returns, so the number of orphaned in-flight requests can never exceed
// the limiter's capacity. When no slot is free it returns errTooManyInFlight
// immediately instead of spawning another goroutine.
func (l *inFlightLimiter) run(ctx context.Context, fetch func() (*HTTPResult, error)) (*HTTPResult, error) {
	select {
	case l.sem <- struct{}{}:
	default:
		return nil, errTooManyInFlight
	}

	type fetchResult struct {
		resp *HTTPResult
		err  error
	}
	ch := make(chan fetchResult, 1)
	go func() {
		r, e := fetch()
		ch <- fetchResult{r, e}
		<-l.sem // release only once the real request has finished
	}()

	select {
	case <-ctx.Done():
		n := outstandingGoroutines.Add(1)
		if n >= int64(cap(l.sem)) {
			slog.Warn("azuretls orphaned goroutines at capacity", "outstanding", n, "cap", cap(l.sem))
		}
		go func() {
			<-ch
			outstandingGoroutines.Add(-1)
		}()
		return nil, ctx.Err()
	case result := <-ch:
		return result.resp, result.err
	}
}

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
	limiter    *inFlightLimiter
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

	return &stealthClient{
		session:    session,
		userAgents: userAgents,
		limiter:    newInFlightLimiter(maxInFlightStealthFetches),
	}, nil
}

func (c *stealthClient) Get(ctx context.Context, reqURL string) (*HTTPResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ua := c.userAgents[rand.IntN(len(c.userAgents))]

	// azuretls does not support context cancellation natively; the request
	// goroutine may outlive the context. The limiter bounds how many such
	// goroutines can be in flight at once (see inFlightLimiter).
	return c.limiter.run(ctx, func() (*HTTPResult, error) {
		resp, err := c.session.Get(reqURL, azuretls.OrderedHeaders{
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
		if err != nil {
			return nil, err
		}

		header := make(http.Header, len(resp.Header))
		for k, v := range resp.Header {
			header[k] = v
		}

		body := resp.Body
		if len(body) > maxResponseSize {
			body = body[:maxResponseSize]
		}

		return &HTTPResult{
			Body:       body,
			StatusCode: resp.StatusCode,
			Header:     header,
		}, nil
	})
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
	defer func() { _ = resp.Body.Close() }()

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
