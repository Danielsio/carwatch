package winwin

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseSize = 10 * 1024 * 1024

// HTTPResult holds the outcome of an HTTP GET request.
type HTTPResult struct {
	Body       []byte
	StatusCode int
	Header     http.Header
}

// Client wraps an HTTP client configured for WinWin requests.
type Client struct {
	httpClient *http.Client
	userAgents []string
}

// NewClient creates an HTTP client with optional proxy support.
func NewClient(userAgents []string, proxy string) (*Client, error) {
	if len(userAgents) == 0 {
		return nil, fmt.Errorf("at least one user agent is required")
	}
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

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		userAgents: userAgents,
	}, nil
}

// Do executes an HTTP request with browser-like headers for winwin.co.il.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	ua := c.userAgents[rand.IntN(len(c.userAgents))]
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "he-IL,he;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")

	return c.httpClient.Do(req)
}

// CloseIdleConnections closes any idle connections on the client's transport.
func (c *Client) CloseIdleConnections() {
	c.httpClient.CloseIdleConnections()
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	reader := io.LimitReader(resp.Body, maxResponseSize)
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer func() { _ = gr.Close() }()
		reader = gr
	}
	return io.ReadAll(reader)
}

// Get performs a GET and returns status, headers, and fully-read body.
func (c *Client) Get(ctx context.Context, reqURL string) (*HTTPResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	header := make(http.Header, len(resp.Header))
	for k, v := range resp.Header {
		header[k] = append([]string(nil), v...)
	}
	return &HTTPResult{Body: body, StatusCode: resp.StatusCode, Header: header}, nil
}
