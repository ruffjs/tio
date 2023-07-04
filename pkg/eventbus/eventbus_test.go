package eventbus_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/eventbus"
)

func TestSubPub(t *testing.T) {
	wg := sync.WaitGroup{}
	sub1Want := []string{"hi one"}
	sub2Want := []string{"hi one", "hi two"}
	var sub1Got []string
	var sub2Got []string

	eventBus := eventbus.NewEventBus[string]()

	ch1 := eventBus.Subscribe("myEvent")
	ch2 := eventBus.Subscribe("myEvent")
	go func() {
		for {
			message := <-ch1
			fmt.Println("Subscriber 1:", message)
			sub1Got = append(sub1Got, message)
			wg.Done()
		}
	}()

	go func() {
		for {
			message := <-ch2
			fmt.Println("Subscriber 2:", message)
			sub2Got = append(sub2Got, message)
			wg.Done()
		}
	}()

	wg.Add(2)
	eventBus.Publish("myEvent", "hi one")

	eventBus.Unsubscribe("myEvent", ch1)

	wg.Add(1)
	eventBus.Publish("myEvent", "hi two")
	wg.Wait()

	require.Equal(t, sub1Want, sub1Got)
	require.Equal(t, sub2Want, sub2Got)
}
