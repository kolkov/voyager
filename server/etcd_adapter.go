package server

import (
	"context"
	"encoding/json"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdAdapter provides abstraction for ETCD operations
type EtcdAdapter struct {
	client *clientv3.Client
}

// NewEtcdAdapter creates new ETCD adapter
func NewEtcdAdapter(endpoints []string) (*EtcdAdapter, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &EtcdAdapter{client: cli}, nil
}

// Put stores data with TTL
func (e *EtcdAdapter) Put(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}

	leaseResp, err := e.client.Grant(ctx, int64(ttl.Seconds()))
	if err != nil {
		return err
	}

	_, err = e.client.Put(ctx, key, string(jsonData), clientv3.WithLease(leaseResp.ID))
	return err
}

// GetPrefix retrieves data by key prefix
func (e *EtcdAdapter) GetPrefix(ctx context.Context, prefix string) (map[string][]byte, error) {
	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = kv.Value
	}
	return result, nil
}

// Delete removes key
func (e *EtcdAdapter) Delete(ctx context.Context, key string) error {
	_, err := e.client.Delete(ctx, key)
	return err
}

// Close releases ETCD connection
func (e *EtcdAdapter) Close() error {
	return e.client.Close()
}
