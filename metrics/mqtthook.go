package metrics

import (
	"bytes"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/system"
	"github.com/prometheus/client_golang/prometheus"
)

type OpenMetricsHook struct {
	mqtt.HookBase
	lastInfo *system.Info
}

func (o *OpenMetricsHook) ID() string {
	return "open_metrics"
}

func (o *OpenMetricsHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnSysInfoTick,
	}, []byte{b})
}

func (o *OpenMetricsHook) OnSysInfoTick(info *system.Info) {
	// Update Gauge metrics (instantaneous values)
	MQTTBrokerClientsConnected.Set(float64(info.ClientsConnected))
	MQTTBrokerClientsDisconnected.Set(float64(info.ClientsDisconnected))
	MQTTBrokerClientsMaximum.Set(float64(info.ClientsMaximum))
	MQTTBrokerClientsTotal.Set(float64(info.ClientsTotal))
	MQTTBrokerRetained.Set(float64(info.Retained))
	MQTTBrokerInflight.Set(float64(info.Inflight))
	MQTTBrokerSubscriptions.Set(float64(info.Subscriptions))
	MQTTBrokerVersion.WithLabelValues(info.Version).Set(1)
	MQTTBrokerStarted.Set(float64(info.Started))
	MQTTBrokerUptime.Set(float64(info.Uptime))

	// Update Counter metrics (cumulative values with delta calculation)
	if o.lastInfo != nil {
		o.updateCounter(MQTTBrokerBytesReceived, info.BytesReceived, o.lastInfo.BytesReceived)
		o.updateCounter(MQTTBrokerBytesSent, info.BytesSent, o.lastInfo.BytesSent)
		o.updateCounter(MQTTBrokerMessagesReceived, info.MessagesReceived, o.lastInfo.MessagesReceived)
		o.updateCounter(MQTTBrokerMessagesSent, info.MessagesSent, o.lastInfo.MessagesSent)
		o.updateCounter(MQTTBrokerMessagesDropped, info.MessagesDropped, o.lastInfo.MessagesDropped)
		o.updateCounter(MQTTBrokerInflightDropped, info.InflightDropped, o.lastInfo.InflightDropped)
		o.updateCounter(MQTTBrokerPacketsReceived, info.PacketsReceived, o.lastInfo.PacketsReceived)
		o.updateCounter(MQTTBrokerPacketsSent, info.PacketsSent, o.lastInfo.PacketsSent)
	}

	o.lastInfo = info.Clone()
}

func (o *OpenMetricsHook) updateCounter(counter prometheus.Counter, current, last int64) {
	if delta := current - last; delta > 0 {
		counter.Add(float64(delta))
	}
}
