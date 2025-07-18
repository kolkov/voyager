// Package client implements load balancing strategies for service discovery
package client

import (
	"math/rand"
	"net"
	"strconv"
	"sync"

	voyagerv1 "github.com/kolkov/voyager/gen/proto/voyager/v1"
)

// LoadBalancer defines the interface for instance selection strategies
type LoadBalancer interface {
	Select(serviceName string, instances []*voyagerv1.Registration) *voyagerv1.Registration
}

// roundRobinBalancer implements round-robin selection strategy
type roundRobinBalancer struct {
	mu    sync.Mutex
	index map[string]int
}

func newRoundRobinBalancer() *roundRobinBalancer {
	return &roundRobinBalancer{
		index: make(map[string]int),
	}
}

// Select chooses the next instance in sequence
func (b *roundRobinBalancer) Select(serviceName string, instances []*voyagerv1.Registration) *voyagerv1.Registration {
	if len(instances) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	idx := b.index[serviceName]
	selected := instances[idx%len(instances)]
	b.index[serviceName] = (idx + 1) % len(instances)

	return selected
}

// randomBalancer implements random selection strategy
type randomBalancer struct{}

func newRandomBalancer() *randomBalancer {
	return &randomBalancer{}
}

// Select chooses a random instance
func (b *randomBalancer) Select(_ string, instances []*voyagerv1.Registration) *voyagerv1.Registration {
	if len(instances) == 0 {
		return nil
	}
	return instances[rand.Intn(len(instances))]
}

// leastConnectionsBalancer selects instance with least active connections
type leastConnectionsBalancer struct {
	pool *ConnectionPool
}

func newLeastConnectionsBalancer(pool *ConnectionPool) *leastConnectionsBalancer {
	return &leastConnectionsBalancer{pool: pool}
}

// Select chooses the instance with the fewest active connections
func (b *leastConnectionsBalancer) Select(_ string, instances []*voyagerv1.Registration) *voyagerv1.Registration {
	if len(instances) == 0 {
		return nil
	}

	var selected *voyagerv1.Registration
	minConns := int64(1<<63 - 1)

	for _, inst := range instances {
		address := net.JoinHostPort(inst.Address, strconv.Itoa(int(inst.Port)))
		conns := b.pool.ConnectionCount(address)

		if conns < minConns {
			minConns = conns
			selected = inst
		}
	}

	return selected
}
