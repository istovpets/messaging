package messaging

import (
	"fmt"
	"sync"
	"time"
)

// DeliveryMode defines how messages are delivered to subscribers
// and what delivery guarantees are provided.
type DeliveryMode int

const (
	// DeliveryHard performs blocking delivery.
	// The sender waits until the message is accepted by the subscriber.
	//
	// Guarantees:
	//   - No message loss.
	//
	// Trade-offs:
	//   - Can block indefinitely if the subscriber is slow or not reading.
	DeliveryHard DeliveryMode = iota

	// DeliverySoft performs non-blocking delivery with retries.
	// If the channel is full, it retries sending with a delay and eventually drops the message.
	//
	// Guarantees:
	//   - Best-effort delivery.
	//   - Message may be lost after retries are exhausted.
	//
	// Trade-offs:
	//   - Does not block the sender.
	//   - Delivery is not guaranteed.
	DeliverySoft

	// DeliveryBounded performs blocking delivery with a timeout.
	// The sender waits until the message is accepted or the timeout expires.
	//
	// Guarantees:
	//   - Message is delivered if the subscriber becomes ready before the timeout.
	//   - Message is dropped on timeout.
	//
	// Trade-offs:
	//   - Bounded blocking (limited wait time).
	//   - Delivery is not guaranteed.
	DeliveryBounded
)

const (
	defRetryDelay = 100 * time.Millisecond
	defRetryCount = 3
	defBufferSize = 1
	defTimeout    = 100 * time.Millisecond
)

type Logger interface {
	Error(msg string, args ...any)
}

type Notifier struct {
	mu         sync.RWMutex
	topics     map[string][]chan any
	closed     bool
	wg         sync.WaitGroup
	mode       DeliveryMode
	bufferSize int
	retryCount int
	retryDelay time.Duration
	timeout    time.Duration
	logger     Logger
}

type Option func(*Notifier)

func WithDeliveryMode(mode DeliveryMode) Option {
	return func(n *Notifier) {
		n.mode = mode
	}
}

func WithBuffer(size int) Option {
	return func(n *Notifier) {
		n.bufferSize = size
	}
}

func WithRetry(count int, delay time.Duration) Option {
	return func(n *Notifier) {
		n.retryCount = count
		n.retryDelay = delay
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(n *Notifier) {
		n.timeout = timeout
	}
}

func WithLogger(l Logger) Option {
	return func(n *Notifier) {
		n.logger = l
	}
}

// NewNotifier creates a new Notifier instance with the provided options.
// If no options are specified, default configuration values are used.
func NewNotifier(opts ...Option) *Notifier {
	n := &Notifier{
		topics:     make(map[string][]chan any),
		bufferSize: defBufferSize,
		mode:       DeliverySoft,
		retryCount: defRetryCount,
		retryDelay: defRetryDelay,
		timeout:    defTimeout,
	}

	for _, opt := range opts {
		opt(n)
	}

	return n
}

func (n *Notifier) subscribeChan(topic string) (chan any, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil, fmt.Errorf("notifier is closed")
	}

	ch := make(chan any, n.bufferSize)

	oldChnls := n.topics[topic]

	// copy-on-write to avoid data races with concurrent readers
	newChnls := make([]chan any, len(oldChnls)+1)
	copy(newChnls, oldChnls)
	newChnls[len(oldChnls)] = ch

	n.topics[topic] = newChnls

	return ch, nil
}

// It returns an unsubscribe function that can be called to stop receiving messages.
//
// The handler is executed in a separate goroutine and will receive messages
// published to the topic until unsubscribed or the notifier is closed.
func (n *Notifier) Subscribe(topic string, handler func(any)) (func(), error) {
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}

	ch, err := n.subscribeChan(topic)
	if err != nil {
		return nil, err
	}

	n.wg.Go(func() {
		for msg := range ch {
			func() {
				defer func() {
					if r := recover(); r != nil {
						n.logError(fmt.Sprintf("handler panic: %v", r), topic)
					}
				}()
				handler(msg)
			}()
		}
	})

	return func() {
		n.unsubscribe(topic, ch)
	}, nil
}

// Publish sends a message to all subscribers of the given topic.
// Delivery behavior depends on the configured DeliveryMode.
//
// Returns an error if the notifier is closed.
func (n *Notifier) Publish(topic string, message any) error {
	n.mu.RLock()
	if n.closed {
		n.mu.RUnlock()
		return fmt.Errorf("notifier is closed")
	}

	chnls := n.topics[topic]
	n.mu.RUnlock()

	for _, ch := range chnls {
		n.deliver(topic, ch, message)
	}

	return nil
}

func (n *Notifier) deliver(topic string, ch chan any, message any) {
	defer func() {
		if r := recover(); r != nil {
			// ignore send to closed channel
			n.logError("deliver failed: channel is closed", topic)
		}
	}()

	switch n.mode {
	case DeliveryHard: // blocking
		ch <- message

	case DeliverySoft: // retry + drop
		sent := false

		for i := 0; i <= n.retryCount; i++ {
			select {
			case ch <- message:
				sent = true
			default:
				if i < n.retryCount && n.retryDelay > 0 {
					time.Sleep(n.retryDelay)
				}
			}

			if sent {
				break
			}
		}

		if !sent {
			n.logError("deliver failed: message dropped after retries", topic)
		}

	case DeliveryBounded: // timeout
		if n.timeout <= 0 {
			// fallback to soft
			select {
			case ch <- message:
				return
			default:
				n.logError("deliver failed: message dropped (no timeout)", topic)
			}

			return
		}

		select {
		case ch <- message:
			return
		case <-time.After(n.timeout):
			n.logError("deliver failed: timeout exceeded", topic)
		}
	}
}

func (n *Notifier) unsubscribe(topic string, ch chan any) {
	n.mu.Lock()
	defer n.mu.Unlock()

	oldChnls := n.topics[topic]

	for i, c := range oldChnls {
		if c == ch {
			newChnls := make([]chan any, 0, len(oldChnls)-1)
			newChnls = append(newChnls, oldChnls[:i]...)
			newChnls = append(newChnls, oldChnls[i+1:]...)

			n.topics[topic] = newChnls
			close(ch)

			return
		}
	}
}

// UnsubscribeAll removes all subscribers for the given topic
// and closes their channels.
func (n *Notifier) UnsubscribeAll(topic string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, ch := range n.topics[topic] {
		close(ch)
	}

	delete(n.topics, topic)
}

// Close shuts down the notifier and closes all subscriber channels.
// After calling Close, the notifier cannot be used anymore.
func (n *Notifier) Close() {
	n.mu.Lock()

	if n.closed {
		return
	}

	for _, chs := range n.topics {
		for _, ch := range chs {
			close(ch)
		}
	}

	n.topics = nil
	n.closed = true

	n.mu.Unlock()

	n.wg.Wait()
}

func (n *Notifier) logError(msg string, topic string) {
	if n.logger != nil {
		n.logger.Error(msg, "topic", topic)
	}
}
