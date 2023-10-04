package ctrl

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

	// The build version
	promVersion *prometheus.GaugeVec

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
	promMessagesProcessedIDLast prometheus.Gauge
	// Prometheus metrics for the total count of stalled
	// messages in the ringbuffer.
	promRingbufferStalledMessagesTotal prometheus.Counter
	// Prometheus metrics for current messages in memory buffer.
	promInMemoryBufferMessagesCurrent prometheus.Gauge
	// Prometheus metrics for current messages delivered by a
	// user into the system.
	promUserMessagesTotal prometheus.Counter
	// Metrics for nats messages delivered total.
	promNatsDeliveredTotal prometheus.Counter
	// Metrics for messages that failed to get ack replies.
	promNatsMessagesFailedACKsTotal prometheus.Counter
	// Metrics for messages that missed to get ack replies.
	promNatsMessagesMissedACKsTotal prometheus.Counter
	// Metrics for received error messages
	promErrorMessagesReceivedTotal prometheus.Counter
	// Metrics for sent error messages
	promErrorMessagesSentTotal prometheus.Counter
	// Metrics for sent info messages
	promInfoMessagesSentTotal prometheus.Counter
	// Metrics for the amount of messages currently in db.
	promDBMessagesCurrent prometheus.Gauge
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

	m.promVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ctrl_build_version",
		Help: "Build version of ctrl",
	}, []string{"version"},
	)
	m.promRegistry.MustRegister(m.promVersion)

	m.promProcessesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ctrl_processes_total",
		Help: "The current number of total running processes",
	})
	m.promRegistry.MustRegister(m.promProcessesTotal)

	m.promProcessesAllRunning = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ctrl_processes_all_running",
		Help: "Name of the running processes",
	}, []string{"processName"},
	)
	m.promRegistry.MustRegister(m.promProcessesAllRunning)

	m.promHelloNodesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ctrl_hello_nodes_total",
		Help: "The current number of total nodes who have said hello",
	})
	m.promRegistry.MustRegister(m.promHelloNodesTotal)

	m.promHelloNodesContactLast = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ctrl_hello_node_contact_last",
		Help: "Name of the nodes who have said hello",
	}, []string{"nodeName"})
	m.promRegistry.MustRegister(m.promHelloNodesContactLast)

	m.promMessagesProcessedIDLast = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ctrl_messages_processed_id_last",
		Help: "The last processed id in key value/store db",
	})
	m.promRegistry.MustRegister(m.promMessagesProcessedIDLast)

	m.promRingbufferStalledMessagesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_ringbuffer_stalled_messages_total",
		Help: "Number of stalled messages in ringbuffer",
	})
	m.promRegistry.MustRegister(m.promRingbufferStalledMessagesTotal)

	m.promInMemoryBufferMessagesCurrent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ctrl_in_memory_buffer_messages_current",
		Help: "The current value of messages in memory buffer",
	})
	m.promRegistry.MustRegister(m.promInMemoryBufferMessagesCurrent)

	// Register som metrics for messages delivered by users into the system.
	m.promUserMessagesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_user_messages_total",
		Help: "Number of total messages delivered by users into the system",
	})
	m.promRegistry.MustRegister(m.promUserMessagesTotal)

	m.promNatsDeliveredTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_nats_delivered_total",
		Help: "Number of total messages delivered by nats",
	})
	m.promRegistry.MustRegister(m.promNatsDeliveredTotal)

	m.promNatsMessagesFailedACKsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_nats_messages_failed_acks_total",
		Help: "Number of messages that never received an ack total",
	})
	m.promRegistry.MustRegister(m.promNatsMessagesFailedACKsTotal)

	m.promNatsMessagesMissedACKsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_nats_messages_missed_acks_total",
		Help: "Number of messages missed receiving an ack total",
	})
	m.promRegistry.MustRegister(m.promNatsMessagesMissedACKsTotal)

	m.promErrorMessagesReceivedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_error_messages_received_total",
		Help: "Number of error messages received total",
	})
	m.promRegistry.MustRegister(m.promErrorMessagesReceivedTotal)

	m.promErrorMessagesSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_error_messages_sent_total",
		Help: "Number of error messages sent total",
	})
	m.promRegistry.MustRegister(m.promErrorMessagesSentTotal)

	m.promInfoMessagesSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ctrl_info_messages_sent_total",
		Help: "Number of info messages sent total",
	})
	m.promRegistry.MustRegister(m.promInfoMessagesSentTotal)

	m.promDBMessagesCurrent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ctrl_db_messages_current",
		Help: "The current value messages in database",
	})
	m.promRegistry.MustRegister(m.promDBMessagesCurrent)

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
