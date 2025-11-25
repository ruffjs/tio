package metrics

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTPRequestDuration tracks HTTP request duration in seconds
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTPRequestsTotal tracks total number of HTTP requests
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTPRequestSize tracks HTTP request size in bytes
	HTTPRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "route"},
	)

	// HTTPResponseSize tracks HTTP response size in bytes
	HTTPResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "route"},
	)

	// MQTT broker system info metrics
	MQTTBrokerVersion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_version_info",
			Help: "MQTT broker version information",
		},
		[]string{"version"},
	)

	MQTTBrokerStarted = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_started_timestamp_seconds",
			Help: "The time the server started in unix seconds",
		},
	)

	MQTTBrokerUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_uptime_seconds",
			Help: "The number of seconds the server has been online",
		},
	)

	MQTTBrokerBytesReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_bytes_received_total",
			Help: "Total number of bytes received since the broker started",
		},
	)

	MQTTBrokerBytesSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_bytes_sent_total",
			Help: "Total number of bytes sent since the broker started",
		},
	)

	MQTTBrokerClientsConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_clients_connected",
			Help: "Number of currently connected clients",
		},
	)

	MQTTBrokerClientsDisconnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_clients_disconnected",
			Help: "Total number of persistent clients that are registered but currently disconnected",
		},
	)

	MQTTBrokerClientsMaximum = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_clients_maximum",
			Help: "Maximum number of active clients that have been connected",
		},
	)

	MQTTBrokerClientsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_clients_total",
			Help: "Total number of connected and disconnected clients with a persistent session",
		},
	)

	MQTTBrokerMessagesReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_messages_received_total",
			Help: "Total number of publish messages received",
		},
	)

	MQTTBrokerMessagesSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_messages_sent_total",
			Help: "Total number of publish messages sent",
		},
	)

	MQTTBrokerMessagesDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_messages_dropped_total",
			Help: "Total number of publish messages dropped to slow subscriber",
		},
	)

	MQTTBrokerRetained = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_retained_messages",
			Help: "Total number of retained messages active on the broker",
		},
	)

	MQTTBrokerInflight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_inflight_messages",
			Help: "The number of messages currently in-flight",
		},
	)

	MQTTBrokerInflightDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_inflight_dropped_total",
			Help: "The number of inflight messages which were dropped",
		},
	)

	MQTTBrokerSubscriptions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mqtt_broker_subscriptions",
			Help: "Total number of subscriptions active on the broker",
		},
	)

	MQTTBrokerPacketsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_packets_received_total",
			Help: "The total number of publish messages received",
		},
	)

	MQTTBrokerPacketsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_broker_packets_sent_total",
			Help: "Total number of messages of any type sent since the broker started",
		},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(
		// HTTP metrics
		HTTPRequestDuration,
		HTTPRequestsTotal,
		HTTPRequestSize,
		HTTPResponseSize,

		// MQTT broker metrics
		MQTTBrokerVersion,
		MQTTBrokerStarted,
		MQTTBrokerUptime,
		MQTTBrokerBytesReceived,
		MQTTBrokerBytesSent,
		MQTTBrokerClientsConnected,
		MQTTBrokerClientsDisconnected,
		MQTTBrokerClientsMaximum,
		MQTTBrokerClientsTotal,
		MQTTBrokerMessagesReceived,
		MQTTBrokerMessagesSent,
		MQTTBrokerMessagesDropped,
		MQTTBrokerRetained,
		MQTTBrokerInflight,
		MQTTBrokerInflightDropped,
		MQTTBrokerSubscriptions,
		MQTTBrokerPacketsReceived,
		MQTTBrokerPacketsSent,
	)
}

var metricsHandler = promhttp.Handler()

func Service() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/metrics").
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("/").To(func(req *restful.Request, resp *restful.Response) {
		metricsHandler.ServeHTTP(resp.ResponseWriter, req.Request)
	}))
	return ws
}
