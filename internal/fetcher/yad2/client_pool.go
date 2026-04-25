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
	if c, ok := p.clients[proxy]; ok {
		p.mu.Unlock()
		return c, nil
	}
	p.mu.Unlock()

	c, err := NewClient(p.userAgents, proxy)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.clients[proxy]; ok {
		c.Close()
		return existing, nil
	}
	p.clients[proxy] = c
	return c, nil
}

func (p *ClientPool) Evict(proxy string) {
	p.mu.Lock()
	c, ok := p.clients[proxy]
	if ok {
		delete(p.clients, proxy)
	}
	p.mu.Unlock()
	if ok {
		c.Close()
	}
}

func (p *ClientPool) Close() {
	p.mu.Lock()
	clients := p.clients
	p.clients = make(map[string]HTTPDoer)
	p.mu.Unlock()
	for _, c := range clients {
		c.Close()
	}
}
