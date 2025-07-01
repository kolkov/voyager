package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics definitions
var (
	registrationCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "voyager_registrations_total",
		Help: "Total service registrations",
	}, []string{"service"})

	discoveryCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "voyager_discoveries_total",
		Help: "Total service discoveries",
	}, []string{"service", "status"})

	serviceInstancesGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "voyager_service_instances",
		Help: "Number of service instances",
	}, []string{"service"})

	cacheRefreshCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "voyager_cache_refreshes_total",
		Help: "Total cache refresh operations",
	})

	cacheRefreshErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "voyager_cache_refresh_errors_total",
		Help: "Total cache refresh errors",
	})
)

// MetricsHandler returns Prometheus metrics handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// UpdateServiceMetrics updates service instance metrics
func (s *Server) UpdateServiceMetrics() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.inMemory {
		for service, instances := range s.inMemoryInstances {
			serviceInstancesGauge.WithLabelValues(service).Set(float64(len(instances)))
		}
	} else {
		for service, instances := range s.services {
			serviceInstancesGauge.WithLabelValues(service).Set(float64(len(instances)))
		}
	}
}

// IncRegistrationCounter increments registration counter
func IncRegistrationCounter(service string) {
	registrationCounter.WithLabelValues(service).Inc()
}

// IncDiscoveryCounter increments discovery counter
func IncDiscoveryCounter(service, status string) {
	discoveryCounter.WithLabelValues(service, status).Inc()
}
