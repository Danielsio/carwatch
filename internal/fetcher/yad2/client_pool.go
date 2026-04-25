package yad2

import (
	"log/slog"
	"sync"
)

type ClientPool struct {
	mu         sync.Mutex
	clients    map[string]HTTPDoer
	userAgents []string
	logger     *slog.Logger
}

func NewClientPool(userAgents []string, logger *slog.Logger) *ClientPool {
	return &ClientPool{
		clients:    make(map[string]HTTPDoer),
		userAgents: userAgents,
		logger:     logger,
	}
}

func (p *ClientPool) Get(proxy string) (HTTPDoer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.clients[proxy]; ok {
		return c, nil
	}

	c, err := NewClient(p.userAgents, proxy)
	if err != nil {
		return nil, err
	}
	p.clients[proxy] = c
	return c, nil
}

func (p *ClientPool) Evict(proxy string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.clients[proxy]; ok {
		c.Close()
		delete(p.clients, proxy)
	}
}

func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for proxy, c := range p.clients {
		c.Close()
		delete(p.clients, proxy)
	}
}
