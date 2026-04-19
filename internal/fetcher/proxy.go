package fetcher

import (
	"sync"
	"time"
)

type ProxyPool struct {
	mu      sync.Mutex
	proxies []proxyEntry
	idx     int
}

type proxyEntry struct {
	url       string
	healthy   bool
	cooldown  time.Time
}

const proxyCooldown = 5 * time.Minute

func NewProxyPool(urls []string) *ProxyPool {
	entries := make([]proxyEntry, len(urls))
	for i, u := range urls {
		entries[i] = proxyEntry{url: u, healthy: true}
	}
	return &ProxyPool{proxies: entries}
}

func (p *ProxyPool) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.proxies) == 0 {
		return ""
	}

	for range len(p.proxies) {
		entry := &p.proxies[p.idx]
		p.idx = (p.idx + 1) % len(p.proxies)

		if !entry.healthy && time.Now().After(entry.cooldown) {
			entry.healthy = true
		}

		if entry.healthy {
			return entry.url
		}
	}

	p.proxies[p.idx].healthy = true
	return p.proxies[p.idx].url
}

func (p *ProxyPool) MarkUnhealthy(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.proxies {
		if p.proxies[i].url == url {
			p.proxies[i].healthy = false
			p.proxies[i].cooldown = time.Now().Add(proxyCooldown)
			return
		}
	}
}

func (p *ProxyPool) MarkHealthy(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.proxies {
		if p.proxies[i].url == url {
			p.proxies[i].healthy = true
			return
		}
	}
}
