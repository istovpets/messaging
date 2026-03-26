package main

import (
	"fmt"
	"time"

	"github.com/istovpets/messaging"
)

func main() {
	notifier := messaging.NewNotifier()
	defer notifier.Close()

	notifier.Subscribe("topic1", func(msg any) {
		fmt.Printf("topic1 %v", msg)
	})

	notifier.Subscribe("topic2", func(msg any) {
		println("topic2:", msg)
	})

	notifier.Publish("topic1", "Hello, world!")
	notifier.Publish("topic3", "Hello, world!")
	time.Sleep(time.Second)
}
