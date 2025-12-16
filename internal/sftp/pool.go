package sftp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/pkg/logger"
)

// Pool manages a collection of SFTP clients.
type Pool struct {
	clients map[string]*Client
	configs map[string]config.SFTPConn
	logger  *logger.Logger
	mu      sync.RWMutex
}

// NewPool creates a new connection pool.
func NewPool(connections map[string]config.SFTPConn, log *logger.Logger) *Pool {
	return &Pool{
		clients: make(map[string]*Client),
		configs: connections,
		logger:  log.WithComponent("pool"),
	}
}

// Get returns a connected client for the given connection name.
func (p *Pool) Get(ctx context.Context, name string) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if client exists
	client, exists := p.clients[name]
	if exists && client.IsConnected() {
		return client, nil
	}

	// Get connection config
	cfg, ok := p.configs[name]
	if !ok {
		return nil, fmt.Errorf("unknown connection: %s", name)
	}

	// Create new client if needed
	if !exists {
		client = NewClient(name, cfg, p.logger)
		p.clients[name] = client
	}

	// Connect
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", name, err)
	}

	return client, nil
}

// ConnectAll establishes connections to all configured SFTP servers.
func (p *Pool) ConnectAll(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(p.configs))

	for name := range p.configs {
		wg.Add(1)
		go func(connName string) {
			defer wg.Done()
			if _, err := p.Get(ctx, connName); err != nil {
				errChan <- err
			}
		}(name)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("connection errors: %v", errs)
	}

	return nil
}

// ConnectWithRetry attempts to connect with retry logic.
func (p *Pool) ConnectWithRetry(ctx context.Context, name string, retryInterval time.Duration) (*Client, error) {
	for {
		client, err := p.Get(ctx, name)
		if err == nil {
			return client, nil
		}

		p.logger.Warn().
			Err(err).
			Str("connection", name).
			Dur("retry_interval", retryInterval).
			Msg("Connection failed, retrying...")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
			// Continue retry loop
		}
	}
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for name, client := range p.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	p.clients = make(map[string]*Client)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// Reconnect reconnects a specific connection.
func (p *Pool) Reconnect(ctx context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	client, exists := p.clients[name]
	if !exists {
		cfg, ok := p.configs[name]
		if !ok {
			return fmt.Errorf("unknown connection: %s", name)
		}
		client = NewClient(name, cfg, p.logger)
		p.clients[name] = client
	}

	return client.Reconnect(ctx)
}

// Names returns the names of all configured connections.
func (p *Pool) Names() []string {
	names := make([]string, 0, len(p.configs))
	for name := range p.configs {
		names = append(names, name)
	}
	return names
}

// IsConnected checks if a specific connection is active.
func (p *Pool) IsConnected(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	client, exists := p.clients[name]
	return exists && client.IsConnected()
}
