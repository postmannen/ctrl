package steward

import (
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics are generally used to hold the structure around metrics
// handling
type metrics struct {
	// The channel to pass metrics that should be processed.
	promRegistry *prometheus.Registry
	// host and port where prometheus metrics will be exported.
	hostAndPort string

	// --- Processes
	// Prometheus metrics for total processes.
	promProcessesTotal prometheus.Gauge
	// Prometheus metrics for vector of process names.
	promProcessesAllRunning *prometheus.GaugeVec

	// --- Methods
	// Prometheus metrics for number of hello nodes.
	promHelloNodesTotal prometheus.Gauge
	// Prometheus metrics for the vector of hello nodes.
	promHelloNodesContactLast *prometheus.GaugeVec

	// --- Ringbuffer
	// Prometheus metrics for the last processed DB id in key
	// value store.
	promMessagesProcessedTotal prometheus.Gauge
	// Prometheus metrics for the total count of stalled
	// messages in the ringbuffer.
	promRingbufferStalledMessagesTotal prometheus.Counter
	// Prometheus metrics for current messages in memory buffer
	promInMemoryBufferMessagesCurrent prometheus.Gauge
}

// newMetrics will prepare and return a *metrics.
func newMetrics(hostAndPort string) *metrics {
	reg := prometheus.NewRegistry()
	//prometheus.Unregister(prometheus.NewGoCollector()).
	reg.MustRegister(collectors.NewGoCollector())
	// prometheus.MustRegister(collectors.NewGoCollector()).
	m := metrics{
		promRegistry: reg,
		hostAndPort:  hostAndPort,
	}

	return &m
}

// Start the http interface for Prometheus metrics.
func (m *metrics) start() error {

	//http.Handle("/metrics", promhttp.Handler())
	//http.ListenAndServe(":2112", nil)
	n, err := net.Listen("tcp", m.hostAndPort)
	if err != nil {
		return fmt.Errorf("error: startMetrics: failed to open prometheus listen port: %v", err)
	}
	//mux := http.NewServeMux()
	//mux.Handle("/metrics", promhttp.Handler())

	http.Handle("/metrics", promhttp.HandlerFor(m.promRegistry, promhttp.HandlerOpts{}))

	err = http.Serve(n, nil)
	if err != nil {
		return fmt.Errorf("error: startMetrics: failed to start http.Serve: %v", err)
	}

	return nil
}
