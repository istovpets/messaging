package messaging

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewNotifier_DefaultValues(t *testing.T) {
	n := NewNotifier()

	if n.mode != DeliverySoft {
		t.Errorf("expected mode DeliverySoft, got %v", n.mode)
	}
	if n.bufferSize != 1 {
		t.Errorf("expected bufferSize 1, got %d", n.bufferSize)
	}
	if n.retryCount != 0 {
		t.Errorf("expected retryCount 0, got %d", n.retryCount)
	}
	if n.retryDelay != 0 {
		t.Errorf("expected retryDelay 0, got %v", n.retryDelay)
	}
	if n.timeout != 0 {
		t.Errorf("expected timeout 0, got %v", n.timeout)
	}
}

func TestNewNotifier_WithOptions(t *testing.T) {
	n := NewNotifier(
		WithDeliveryMode(DeliveryHard),
		WithBuffer(10),
		WithRetry(3, 100*time.Millisecond),
		WithTimeout(5*time.Second),
	)

	if n.mode != DeliveryHard {
		t.Errorf("expected mode DeliveryHard, got %v", n.mode)
	}
	if n.bufferSize != 10 {
		t.Errorf("expected bufferSize 10, got %d", n.bufferSize)
	}
	if n.retryCount != 3 {
		t.Errorf("expected retryCount 3, got %d", n.retryCount)
	}
	if n.retryDelay != 100*time.Millisecond {
		t.Errorf("expected retryDelay 100ms, got %v", n.retryDelay)
	}
	if n.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", n.timeout)
	}
}

func TestSubscribe_Publish(t *testing.T) {
	n := NewNotifier(WithBuffer(2))
	defer n.Close()

	var received []any
	var mu sync.Mutex

	unsub, err := n.Subscribe("test", func(msg any) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsub()

	if err := n.Publish("test", "message1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := n.Publish("test", "message2"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Errorf("expected 2 messages, got %d", len(received))
	}
	if received[0] != "message1" {
		t.Errorf("expected message1, got %v", received[0])
	}
	if received[1] != "message2" {
		t.Errorf("expected message2, got %v", received[1])
	}
	mu.Unlock()
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	n := NewNotifier()
	defer n.Close()

	var count1, count2 atomic.Int32

	unsub1, err := n.Subscribe("test", func(msg any) {
		count1.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsub1()

	unsub2, err := n.Subscribe("test", func(msg any) {
		count2.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsub2()

	if err := n.Publish("test", "msg"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if count1.Load() != 1 {
		t.Errorf("subscriber1 expected 1 message, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("subscriber2 expected 1 message, got %d", count2.Load())
	}
}

func TestUnsubscribe(t *testing.T) {
	n := NewNotifier()
	defer n.Close()

	var count atomic.Int32

	unsub, err := n.Subscribe("test", func(msg any) {
		count.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	unsub()

	if err := n.Publish("test", "msg2"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 1 {
		t.Errorf("expected 1 message after unsubscribe, got %d", count.Load())
	}
}

func TestUnsubscribeAll(t *testing.T) {
	n := NewNotifier()
	defer n.Close()

	var count1, count2 atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		count1.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	_, err = n.Subscribe("test", func(msg any) {
		count2.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	n.UnsubscribeAll("test")

	if err := n.Publish("test", "msg2"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if count1.Load() != 1 {
		t.Errorf("subscriber1 expected 1 message, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("subscriber2 expected 1 message, got %d", count2.Load())
	}
}

func TestClose(t *testing.T) {
	n := NewNotifier()

	var count atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		count.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	n.Close()

	if err := n.Publish("test", "msg2"); err == nil {
		t.Error("expected error when publishing to closed notifier")
	}

	if _, err := n.Subscribe("test", func(msg any) {}); err == nil {
		t.Error("expected error when subscribing to closed notifier")
	}

	if count.Load() != 1 {
		t.Errorf("expected 1 message before close, got %d", count.Load())
	}
}

func TestDeliveryHard_Blocking(t *testing.T) {
	n := NewNotifier(WithDeliveryMode(DeliveryHard), WithBuffer(1))
	defer n.Close()

	var received atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		time.Sleep(100 * time.Millisecond)
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		if err := n.Publish("test", "msg1"); err != nil {
			t.Errorf("publish failed: %v", err)
		}
		if err := n.Publish("test", "msg2"); err != nil {
			t.Errorf("publish failed: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - blocking delivery worked
	case <-time.After(500 * time.Millisecond):
		t.Error("publish took too long - blocking delivery may not be working")
	}

	time.Sleep(300 * time.Millisecond)

	if received.Load() != 2 {
		t.Errorf("expected 2 messages, got %d", received.Load())
	}
}

func TestDeliverySoft_NonBlocking(t *testing.T) {
	n := NewNotifier(WithDeliveryMode(DeliverySoft), WithBuffer(1))
	defer n.Close()

	var received atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		time.Sleep(200 * time.Millisecond)
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		if err := n.Publish("test", "msg1"); err != nil {
			t.Errorf("publish failed: %v", err)
		}
		if err := n.Publish("test", "msg2"); err != nil {
			t.Errorf("publish failed: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - non-blocking delivery worked
	case <-time.After(100 * time.Millisecond):
		t.Error("publish took too long - soft delivery should be non-blocking")
	}

	time.Sleep(500 * time.Millisecond)
	// With buffer size 1 and slow consumer, first message should be delivered,
	// second might be dropped depending on timing, but at least one should be received
	if received.Load() == 0 {
		t.Error("expected at least one message to be received")
	}
}

func TestDeliveryBounded_WithTimeout(t *testing.T) {
	n := NewNotifier(WithDeliveryMode(DeliveryBounded), WithBuffer(1), WithTimeout(50*time.Millisecond))
	defer n.Close()

	var received atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		time.Sleep(200 * time.Millisecond)
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 message, got %d", received.Load())
	}
}

func TestDeliveryBounded_NoTimeout(t *testing.T) {
	n := NewNotifier(WithDeliveryMode(DeliveryBounded), WithBuffer(1))
	defer n.Close()

	var received atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		time.Sleep(100 * time.Millisecond)
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 message, got %d", received.Load())
	}
}

func TestPublish_NoSubscribers(t *testing.T) {
	n := NewNotifier()
	defer n.Close()

	if err := n.Publish("test", "msg"); err != nil {
		t.Errorf("publish with no subscribers should not error: %v", err)
	}
}

func TestPublish_DifferentTopics(t *testing.T) {
	n := NewNotifier()
	defer n.Close()

	var count1, count2 atomic.Int32

	_, err := n.Subscribe("topic1", func(msg any) {
		count1.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	_, err = n.Subscribe("topic2", func(msg any) {
		count2.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("topic1", "msg"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := n.Publish("topic2", "msg"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if count1.Load() != 1 {
		t.Errorf("topic1 expected 1 message, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("topic2 expected 1 message, got %d", count2.Load())
	}
}

func TestWithLogger(t *testing.T) {
	mockLogger := &mockLogger{}
	n := NewNotifier(WithDeliveryMode(DeliverySoft), WithBuffer(1), WithLogger(mockLogger))
	defer n.Close()

	_, err := n.Subscribe("test", func(msg any) {
		time.Sleep(100 * time.Millisecond)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := n.Publish("test", "msg2"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if mockLogger.errorCount.Load() == 0 {
		t.Error("expected logger to be called for dropped messages")
	}
}

func TestDeliverySoft_RetryAndDrop(t *testing.T) {
	mockLogger := &mockLogger{}
	n := NewNotifier(
		WithDeliveryMode(DeliverySoft),
		WithBuffer(1),
		WithRetry(2, 10*time.Millisecond),
		WithLogger(mockLogger),
	)
	defer n.Close()

	var received atomic.Int32

	_, err := n.Subscribe("test", func(msg any) {
		time.Sleep(50 * time.Millisecond)
		received.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Publish multiple messages quickly - buffer size is 1, slow consumer
	// With retry=2, each message gets 3 attempts (initial + 2 retries)
	if err := n.Publish("test", "msg1"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := n.Publish("test", "msg2"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := n.Publish("test", "msg3"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// At least one message should be received (depending on timing)
	if received.Load() == 0 {
		t.Error("expected at least one message to be received")
	}

	// Logger should have been called for dropped messages
	if mockLogger.errorCount.Load() == 0 {
		t.Error("expected logger to be called for dropped messages")
	}
}

func TestConcurrentSubscribePublish(t *testing.T) {
	n := NewNotifier()
	defer n.Close()

	var count atomic.Int32
	var wgSubscribers sync.WaitGroup
	var wgExit sync.WaitGroup
	doneCh := make(chan struct{})

	// Start subscribers
	for range 10 {
		wgSubscribers.Add(1) // For readiness signaling
		wgExit.Go(func() {
			unsub, err := n.Subscribe("test", func(msg any) {
				count.Add(1)
			})
			if err != nil {
				wgSubscribers.Done() // Still signal readiness (for test simplicity)
				return
			}
			defer unsub()
			// Signal that we're ready and waiting for messages
			wgSubscribers.Done()
			// Wait until we are told to stop
			<-doneCh
		})
	}

	// Wait for all subscribers to be ready (they are blocked on <-doneCh)
	wgSubscribers.Wait()

	// Publish messages
	var wgPublish sync.WaitGroup
	for range 100 {
		wgPublish.Go(func() {
			n.Publish("test", "msg") //nolint:errcheck
		})
	}
	wgPublish.Wait()

	// Tell subscribers to stop
	close(doneCh)

	// Wait for subscribers to exit
	wgExit.Wait()
	time.Sleep(100 * time.Millisecond)

	if count.Load() == 0 {
		t.Error("expected some messages to be received")
	}
}

type mockLogger struct {
	errorCount atomic.Int32
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorCount.Add(1)
}
