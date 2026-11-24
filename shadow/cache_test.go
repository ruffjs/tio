package shadow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidationPreventsStaleCacheFill(t *testing.T) {
	cache := newCache()
	const thingID = "device"
	old := true
	current := false
	readStarted := make(chan struct{})
	resume := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		cache.GetOrLoad(thingID, func() (ShadowWithStatus, bool) {
			close(readStarted)
			<-resume
			return ShadowWithStatus{Connected: &old}, true
		})
	}()
	<-readStarted
	cache.Del(thingID)
	close(resume)
	<-readDone

	value, ok := cache.GetOrLoad(thingID, func() (ShadowWithStatus, bool) {
		return ShadowWithStatus{Connected: &current}, true
	})
	require.True(t, ok)
	require.NotNil(t, value.Connected)
	require.False(t, *value.Connected)
}
