package eventbus

var defaultBus = NewEventBus[any]()

func Subscribe(event string) <-chan any {
	return defaultBus.Subscribe(event)
}

func Unsubscribe(event string, ch <-chan any) {
	defaultBus.Unsubscribe(event, ch)
}

func Publish(event string, message any) {
	defaultBus.Publish(event, message)
}
