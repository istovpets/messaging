package messaging

import (
	"fmt"
	"sync"
	"time"
)

type DeliveryMode int

const (
	DeliveryHard    DeliveryMode = iota // blocking delivery (guaranteed, but risk of freezing)
	DeliverySoft                        // non-blocking with retry (best-effort)
	DeliveryBounded                     // blocking with timeout
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

	newChnls := make([]chan any, len(oldChnls)+1)
	copy(newChnls, oldChnls)
	newChnls[len(oldChnls)] = ch

	n.topics[topic] = newChnls

	return ch, nil
}

func (n *Notifier) Subscribe(topic string, handler func(any)) (func(), error) {
	ch, err := n.subscribeChan(topic)
	if err != nil {
		return nil, err
	}

	go func() {
		for msg := range ch {
			handler(msg)
		}
	}()

	return func() {
		n.unsubscribe(topic, ch)
	}, nil
}

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
			n.logError("send to closed channel", topic)
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
			n.logError("message dropped", topic)
		}

	case DeliveryBounded: // timeout
		if n.timeout <= 0 {
			// fallback to soft
			select {
			case ch <- message:
				return
			default:
				n.logError("message dropped (no timeout set)", topic)
			}

			return
		}

		select {
		case ch <- message:
			return
		case <-time.After(n.timeout):
			n.logError("message delivery timeout", topic)
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

func (n *Notifier) UnsubscribeAll(topic string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, ch := range n.topics[topic] {
		close(ch)
	}

	delete(n.topics, topic)
}

func (n *Notifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

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
}

func (n *Notifier) logError(msg string, topic string) {
	if n.logger != nil {
		n.logger.Error(msg, "topic", topic)
	}
}
