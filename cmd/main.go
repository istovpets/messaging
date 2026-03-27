package main

import (
	"fmt"
	"time"

	"github.com/istovpets/messaging"
)

type StdLogger struct{}

func (l *StdLogger) Error(msg string, args ...any) {
	fmt.Println("[ERROR]", msg, args)
}

func main() {
	notifier1 := messaging.NewNotifier()

	// subscribe
	unsub, _ := notifier1.Subscribe("topic", func(msg any) {
		fmt.Println("received:", msg)
	})

	// publish
	notifier1.Publish("topic", "hello")
	notifier1.Publish("topic", "world")

	time.Sleep(100 * time.Millisecond)

	// unsubscribe
	unsub()

	// another notifier with custom options
	notifier2 := messaging.NewNotifier(
		messaging.WithDeliveryMode(messaging.DeliveryBounded),
		messaging.WithTimeout(200*time.Millisecond),
	)

	notifier2.Subscribe("events", func(msg any) {
		fmt.Println("event:", msg)
	})

	notifier2.Publish("events", "ping")
	notifier2.Publish("events", "pong")

	time.Sleep(100 * time.Millisecond)

	// close
	notifier2.Close()
	notifier1.Close()
}
