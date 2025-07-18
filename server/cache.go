// Package server implements service discovery logic
package server

import (
	"context"
	"encoding/json"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	voyagerv1 "github.com/kolkov/voyager/gen/proto/voyager/v1"
)

// refreshCache loads data from etcd into the in-memory cache
func (s *Server) refreshCache() {
	cacheRefreshCounter.Inc()

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	resp, err := s.etcdClient.Get(ctx, "/services/", clientv3.WithPrefix())
	if err != nil {
		// Handle context cancellation gracefully
		if err == context.Canceled {
			log.Println("Cache refresh canceled, server shutting down")
			return
		}

		cacheRefreshErrors.Inc()
		log.Printf("Failed to refresh cache: %v", err)
		return
	}

	newCache := make(map[string]map[string]*voyagerv1.Registration)

	for _, kv := range resp.Kvs {
		var reg voyagerv1.Registration
		if err := json.Unmarshal(kv.Value, &reg); err != nil {
			log.Printf("Failed to unmarshal registration: %v", err)
			continue
		}

		if _, exists := newCache[reg.ServiceName]; !exists {
			newCache[reg.ServiceName] = make(map[string]*voyagerv1.Registration)
		}

		newCache[reg.ServiceName][reg.InstanceId] = &reg
	}

	s.mu.Lock()
	s.services = newCache
	s.mu.Unlock()
}

// startCacheRefresher starts periodic cache refresher
func (s *Server) startCacheRefresher() {
	if s.inMemory {
		return
	}

	ticker := time.NewTicker(s.cacheTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.refreshCache()
		case <-s.ctx.Done():
			log.Println("Stopping cache refresher, server shutting down")
			return
		}
	}
}
