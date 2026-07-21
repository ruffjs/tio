package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMqttSubscriptionToNatsSubjects(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		want    []string
		wantErr bool
	}{
		{name: "wildcard hash with parent", filter: "foo/#", want: []string{"foo", "foo.>"}},
		{name: "hash alone", filter: "#", want: []string{">"}},
		{name: "single level wildcard", filter: "foo/+/bar", want: []string{"foo.*.bar"}},
		{name: "exact topic", filter: "foo/bar/baz", want: []string{"foo.bar.baz"}},
		{name: "iothub topic", filter: "$iothub/things/dev1/shadow/get", want: []string{"$iothub.things.dev1.shadow.get"}},
		{name: "single level", filter: "foo", want: []string{"foo"}},
		{name: "plus at end", filter: "foo/+", want: []string{"foo.*"}},
		{name: "plus at start", filter: "+/bar", want: []string{"*.bar"}},
		{name: "all pluses", filter: "+/+/+", want: []string{"*.*.*"}},
		{name: "deep hash", filter: "a/b/c/#", want: []string{"a.b.c", "a.b.c.>"}},

		// invalid
		{name: "empty", filter: "", wantErr: true},
		{name: "leading slash", filter: "/foo", wantErr: true},
		{name: "trailing slash", filter: "foo/", wantErr: true},
		{name: "empty level", filter: "foo//bar", wantErr: true},
		{name: "dot in level", filter: "foo/b.ar", wantErr: true},
		{name: "whitespace in level", filter: "foo/b ar", wantErr: true},
		{name: "nats star in mqtt", filter: "foo/*/bar", wantErr: true},
		{name: "nats gt in mqtt", filter: "foo/>/bar", wantErr: true},
		{name: "plus not full level", filter: "foo/b+ar", wantErr: true},
		{name: "hash not last level", filter: "foo/#/bar", wantErr: true},
		{name: "hash not last with trailing", filter: "foo#/bar", wantErr: true},
		{name: "plus mixed chars", filter: "foo/+bar", wantErr: true},
		{name: "only slash", filter: "/", wantErr: true},
		{name: "tab in level", filter: "foo/\tbar", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MqttSubscriptionToNatsSubjects(tt.filter)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMqttPublishTopicToNatsSubject(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		want    string
		wantErr bool
	}{
		{name: "exact topic", topic: "foo/bar/baz", want: "foo.bar.baz"},
		{name: "single level", topic: "foo", want: "foo"},
		{name: "iothub topic", topic: "$iothub/things/dev1/shadow/get", want: "$iothub.things.dev1.shadow.get"},

		// invalid
		{name: "empty", topic: "", wantErr: true},
		{name: "leading slash", topic: "/foo", wantErr: true},
		{name: "trailing slash", topic: "foo/", wantErr: true},
		{name: "empty level", topic: "foo//bar", wantErr: true},
		{name: "dot in level", topic: "foo/b.ar", wantErr: true},
		{name: "whitespace", topic: "foo/b ar", wantErr: true},
		{name: "has plus wildcard", topic: "foo/+/bar", wantErr: true},
		{name: "has hash wildcard", topic: "foo/#", wantErr: true},
		{name: "has nats star", topic: "foo/*/bar", wantErr: true},
		{name: "has nats gt", topic: "foo/>/bar", wantErr: true},
		{name: "plus mixed", topic: "foo/b+ar", wantErr: true},
		{name: "hash not last", topic: "foo/#/bar", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MqttPublishTopicToNatsSubject(tt.topic)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestNatsSubjectToMqttTopic(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		want    string
		wantErr bool
	}{
		{name: "exact topic", subject: "foo.bar.baz", want: "foo/bar/baz"},
		{name: "single level", subject: "foo", want: "foo"},
		{name: "iothub topic", subject: "$iothub.things.dev1.shadow.get", want: "$iothub/things/dev1/shadow/get"},

		// invalid
		{name: "empty", subject: "", wantErr: true},
		{name: "leading dot", subject: ".foo", wantErr: true},
		{name: "trailing dot", subject: "foo.", wantErr: true},
		{name: "empty level", subject: "foo..bar", wantErr: true},
		{name: "slash in level", subject: "foo/b.ar", wantErr: true},
		{name: "whitespace", subject: "foo.b ar", wantErr: true},
		{name: "has nats star", subject: "foo.*.bar", wantErr: true},
		{name: "has nats gt", subject: "foo.>", wantErr: true},
		{name: "only dot", subject: ".", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NatsSubjectToMqttTopic(tt.subject)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	topics := []string{
		"foo/bar/baz",
		"foo",
		"$iothub/things/dev1/shadow/get",
		"a/b/c/d/e",
	}
	for _, topic := range topics {
		t.Run(topic, func(t *testing.T) {
			subject, err := MqttPublishTopicToNatsSubject(topic)
			require.NoError(t, err)
			got, err := NatsSubjectToMqttTopic(subject)
			require.NoError(t, err)
			assert.Equal(t, topic, got)
		})
	}
}
