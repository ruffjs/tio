package nats

import (
	"log/slog"

	"ruff.io/tio/connector"
)

// Forward publishes a device uplink to NATS. It is egress-only and never
// blocks: conversion or publish failures are logged at debug and dropped.
func (b *Bridge) Forward(topic string, payload []byte) {
	if b.appConn == nil {
		return
	}
	subject, err := subjectOf(topic)
	if err != nil {
		slog.Debug("bridge forward: invalid topic", "topic", topic, "error", err)
		return
	}
	if err := b.appConn.Publish(subject, payload); err != nil {
		slog.Debug("bridge forward publish", "subject", subject, "error", err)
	}
}

// ForwardPresence publishes a single presence message for a thing.
func (b *Bridge) ForwardPresence(evt connector.PresenceEvent, payload []byte) {
	if b.appConn == nil {
		return
	}
	subject := presenceSubject(evt.ThingId)
	if err := b.appConn.Publish(subject, payload); err != nil {
		slog.Debug("bridge forward presence publish", "subject", subject, "error", err)
	}
}

func subjectOf(topic string) (string, error) {
	return MqttPublishTopicToNatsSubject(topic)
}

func presenceSubject(thingId string) string {
	return "tio." + thingId + ".presence"
}
