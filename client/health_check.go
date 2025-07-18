// Package client implements load balancing strategies for service discovery
package client

import (
	"context"
	"log"
	"time"

	voyagerv1 "github.com/kolkov/voyager/gen/proto/voyager/v1"
)

// startHealthChecks initiates periodic health checks
func (c *Client) startHealthChecks() {
	c.healthMutex.Lock()
	defer c.healthMutex.Unlock()

	if c.healthCheckCtx != nil {
		return
	}

	interval := c.options.HealthCheckInterval
	if interval == 0 {
		interval = c.options.TTL / 3
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
	}

	log.Printf("Starting health checks for service %s, instance %s, interval: %v",
		c.serviceName, c.instanceID, interval)

	ctx, cancel := context.WithCancel(context.Background())
	c.healthCheckCtx = ctx
	c.healthCheckCancel = cancel

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.sendHealthCheck()
			case <-ctx.Done():
				log.Printf("Health checks stopped for service %s instance %s",
					c.serviceName, c.instanceID)
				return
			}
		}
	}()
}

// sendHealthCheck performs a single health check request
func (c *Client) sendHealthCheck() {
	if c.serviceName == "" || c.instanceID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := c.discoverySvc.HealthCheck(ctx, &voyagerv1.HealthRequest{
		ServiceName: c.serviceName,
		InstanceId:  c.instanceID,
	})

	if err != nil {
		log.Printf("Health check failed for service %s instance %s: %v",
			c.serviceName, c.instanceID, err)
		c.reregister()
	}
}

// stopHealthChecks terminates health check routines
func (c *Client) stopHealthChecks() {
	c.healthMutex.Lock()
	defer c.healthMutex.Unlock()

	if c.healthCheckCancel != nil {
		c.healthCheckCancel()
		c.healthCheckCancel = nil
		c.healthCheckCtx = nil
	}
}

// reregister attempts to re-register the service
func (c *Client) reregister() {
	log.Printf("Attempting to re-register service %s instance %s",
		c.serviceName, c.instanceID)

	if c.serviceName == "" {
		return
	}

	if err := c.Register(c.serviceName, c.address, c.port, nil); err != nil {
		log.Printf("Re-registration failed: %v", err)
	} else {
		log.Printf("Service %s re-registered successfully", c.serviceName)
	}
}
