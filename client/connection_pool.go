package client

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ConnectionPooler defines the interface for connection pooling
type ConnectionPooler interface {
	Get(ctx context.Context, address string) (*grpc.ClientConn, error)
	Release(address string)
	Close()
	ConnectionCount(address string) int64
}

// ConnectionPool implements a gRPC connection pool
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*pooledConnection
	opts  *Options
}

type pooledConnection struct {
	*grpc.ClientConn
	refCount int64
}

func (pc *pooledConnection) Close() {
	if atomic.AddInt64(&pc.refCount, -1) <= 0 {
		pc.ClientConn.Close()
	}
}

func (pc *pooledConnection) GetState() connectivity.State {
	return pc.ClientConn.GetState()
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(opts *Options) *ConnectionPool {
	return &ConnectionPool{
		conns: make(map[string]*pooledConnection),
		opts:  opts,
	}
}

// Get returns a connection from the pool or creates a new one
func (p *ConnectionPool) Get(ctx context.Context, address string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	if pc, exists := p.conns[address]; exists {
		atomic.AddInt64(&pc.refCount, 1)
		p.mu.RUnlock()
		return pc.ClientConn, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if pc, exists := p.conns[address]; exists {
		atomic.AddInt64(&pc.refCount, 1)
		return pc.ClientConn, nil
	}

	creds, err := getTransportCredentials(p.opts)
	if err != nil {
		return nil, err
	}

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	}

	if p.opts.DialFunc != nil {
		dialOptions = append(dialOptions, grpc.WithContextDialer(p.opts.DialFunc))
	}

	ctx, cancel := context.WithTimeout(ctx, p.opts.ConnectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, dialOptions...)
	if err != nil {
		return nil, err
	}

	pc := &pooledConnection{
		ClientConn: conn,
		refCount:   1,
	}
	p.conns[address] = pc

	go p.monitorConnection(address, pc)
	return conn, nil
}

// Release decreases reference count for a connection
func (p *ConnectionPool) Release(address string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if pc, exists := p.conns[address]; exists {
		atomic.AddInt64(&pc.refCount, -1)
	}
}

// ConnectionCount returns active connection count for an address
func (p *ConnectionPool) ConnectionCount(address string) int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if pc, exists := p.conns[address]; exists {
		return atomic.LoadInt64(&pc.refCount)
	}
	return 0
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pc := range p.conns {
		pc.Close()
	}
	p.conns = make(map[string]*pooledConnection)
}

// monitorConnection watches connection state and cleans up when idle
func (p *ConnectionPool) monitorConnection(address string, pc *pooledConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if atomic.LoadInt64(&pc.refCount) == 0 && pc.GetState() == connectivity.Ready {
			p.mu.Lock()
			if atomic.LoadInt64(&pc.refCount) == 0 {
				pc.Close()
				delete(p.conns, address)
				p.mu.Unlock()
				return
			}
			p.mu.Unlock()
		}
	}
}
