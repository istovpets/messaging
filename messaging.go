package messaging

import (
	"fmt"
	"sync"
)

type Notifier struct {
	mu     sync.RWMutex
	topics map[string][]chan any
	closed bool
}

func NewNotifier() *Notifier {
	return &Notifier{
		topics: make(map[string][]chan any),
	}
}

func (n *Notifier) subscribeChan(topic string) (chan any, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil, fmt.Errorf("notifier is closed")
	}

	ch := make(chan any, 1)

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
		func(c chan any) {
			defer func() {
				if r := recover(); r != nil {
					// ignore send to closed channel
				}
			}()

			select {
			case c <- message:
			default:
			}
		}(ch)
	}

	return nil
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
