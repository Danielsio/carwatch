package yad2

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	httpClient *http.Client
	userAgents []string
}

func NewClient(userAgents []string, proxy string) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

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

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	ua := c.userAgents[rand.IntN(len(c.userAgents))]
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "he-IL,he;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")

	return c.httpClient.Do(req)
}
