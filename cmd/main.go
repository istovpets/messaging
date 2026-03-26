package main

import (
	"fmt"
	"time"

	"github.com/istovpets/messaging"
)

func main() {
	notifier := messaging.NewNotifier()
	defer notifier.Close()

	if _, err := notifier.Subscribe("topic1", func(msg any) {
		fmt.Printf("topic1 %v", msg)
	}); err != nil {
		fmt.Printf("subscribe error: %v\n", err)
		return
	}

	if _, err := notifier.Subscribe("topic2", func(msg any) {
		println("topic2:", msg)
	}); err != nil {
		fmt.Printf("subscribe error: %v\n", err)
		return
	}

	if err := notifier.Publish("topic1", "Hello, world!"); err != nil {
		fmt.Printf("publish error: %v\n", err)
	}

	if err := notifier.Publish("topic3", "Hello, world!"); err != nil {
		fmt.Printf("publish error: %v\n", err)
	}

	time.Sleep(time.Second)
}
