package client

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// BalancerStrategy defines load balancing strategy types
type BalancerStrategy int

const (
	// RoundRobin selects instances in sequential order
	RoundRobin BalancerStrategy = iota
	// Random selects instances randomly
	Random
	// LeastConnections selects instance with least active connections
	LeastConnections
)

// Options holds configuration options for the Client
type Options struct {
	TTL                 time.Duration
	Insecure            bool
	TLSConfig           *tls.Config
	BalancerStrategy    BalancerStrategy
	ConnectionTimeout   time.Duration
	MaxRetries          int
	RetryDelay          time.Duration
	HealthCheckInterval time.Duration
	DialFunc            func(context.Context, string) (net.Conn, error)
}

// Option configures the Client
type Option func(*Options)

// WithTTL sets cache TTL
func WithTTL(ttl time.Duration) Option {
	return func(o *Options) {
		o.TTL = ttl
	}
}

// WithInsecure disables transport security
func WithInsecure() Option {
	return func(o *Options) {
		o.Insecure = true
	}
}

// WithTLSConfig sets TLS configuration
func WithTLSConfig(cfg *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = cfg
	}
}

// WithBalancerStrategy sets load balancing strategy
func WithBalancerStrategy(strategy BalancerStrategy) Option {
	return func(o *Options) {
		o.BalancerStrategy = strategy
	}
}

// WithConnectionTimeout sets connection timeout
func WithConnectionTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.ConnectionTimeout = timeout
	}
}

// WithRetryPolicy sets retry policy
func WithRetryPolicy(maxRetries int, delay time.Duration) Option {
	return func(o *Options) {
		o.MaxRetries = maxRetries
		o.RetryDelay = delay
	}
}

// WithHealthCheckInterval sets health check interval
func WithHealthCheckInterval(interval time.Duration) Option {
	return func(o *Options) {
		o.HealthCheckInterval = interval
	}
}

// WithDialFunc sets custom dialer function
func WithDialFunc(dialFunc func(context.Context, string) (net.Conn, error)) Option {
	return func(o *Options) {
		o.DialFunc = dialFunc
	}
}

// defaultOptions returns default configuration options
func defaultOptions() *Options {
	return &Options{
		TTL:                 30 * time.Second,
		Insecure:            false,
		BalancerStrategy:    RoundRobin,
		ConnectionTimeout:   5 * time.Second,
		MaxRetries:          5,
		RetryDelay:          2 * time.Second,
		HealthCheckInterval: 0, // Auto-calculated
	}
}
