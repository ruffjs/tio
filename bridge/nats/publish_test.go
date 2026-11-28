package nats

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardSubjects(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		want    string
		wantErr bool
	}{
		{name: "simple topic", topic: "tio/dev1/data", want: "tio.dev1.data"},
		{name: "legacy user topic", topic: "$iothub/user/things/dev1/x", want: "$iothub.user.things.dev1.x"},
		{name: "iothub things topic", topic: "$iothub/things/dev1/ota/status", want: "$iothub.things.dev1.ota.status"},
		{name: "empty", topic: "", wantErr: true},
		{name: "wildcard plus", topic: "tio/+/data", wantErr: true},
		{name: "wildcard hash", topic: "tio/dev1/#", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := subjectOf(tt.topic)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPresenceSubject(t *testing.T) {
	require.Equal(t, "tio.dev1.presence", presenceSubject("dev1"))
}
