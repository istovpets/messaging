# messaging

Lightweight in-memory Pub/Sub (publish–subscribe) implementation in Go.

Designed for simple use cases where you need to broadcast messages between goroutines with configurable delivery semantics.

---

## Features

- Topic-based Pub/Sub
- Multiple subscribers per topic
- Configurable delivery modes:
  - **Hard** — blocking (guaranteed delivery, may block forever)
  - **Soft** — non-blocking with retries (best-effort)
  - **Bounded** — blocking with timeout
- Buffered channels (configurable)
- Safe concurrent usage
- Copy-on-write subscriber lists (no iteration races)

---

## Installation

```bash
go get github.com/istovpets/messaging
```

---

## Quick Example

```go
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
        fmt.Println("topic1:", msg)
    })

    notifier.Publish("topic1", "Hello, world!")

    time.Sleep(time.Second) // wait for async handler
}
```

---

## Usage

### Subscribe

```go
unsubscribe, err := notifier.Subscribe("topic", func(msg any) {
    fmt.Println(msg)
})
if err != nil {
    panic(err)
}

// later
unsubscribe()
```

---

### Publish

```go
err := notifier.Publish("topic", "message")
if err != nil {
    // notifier is closed
}
```

---

## Delivery Modes

### DeliveryHard (default blocking)

```go
messaging.WithDeliveryMode(messaging.DeliveryHard)
```

- Blocks until message is delivered
- Guarantees delivery
- ⚠️ May block forever if subscriber is slow or stuck

---

### DeliverySoft (default)

```go
messaging.WithDeliveryMode(messaging.DeliverySoft)
messaging.WithRetry(3, time.Millisecond*100)
```

- Non-blocking
- Retries configurable number of times
- Drops message if delivery fails
- Suitable for best-effort scenarios

---

### DeliveryBounded

```go
messaging.WithDeliveryMode(messaging.DeliveryBounded)
messaging.WithTimeout(time.Second)
```

- Blocks up to timeout
- Drops message if timeout exceeded

---

## Options

```go
notifier := messaging.NewNotifier(
    messaging.WithBuffer(10),
    messaging.WithDeliveryMode(messaging.DeliverySoft),
    messaging.WithRetry(5, time.Millisecond*50),
    messaging.WithTimeout(time.Second),
    messaging.WithLogger(logger),
)
```

| Option             | Description                         |
|--------------------|-------------------------------------|
| `WithBuffer`       | Channel buffer size per subscriber |
| `WithRetry`        | Retry count and delay (Soft mode)  |
| `WithTimeout`      | Timeout (Bounded mode)             |
| `WithLogger`       | Custom logger                      |

---

## Concurrency Model

- `Publish` is safe for concurrent use
- `Subscribe` / `Unsubscribe` are safe for concurrent use
- Subscriber list uses **copy-on-write**
- Each subscriber runs in its own goroutine

---

## Important Notes

### 1. Send to closed channel

During concurrent `Publish` and `Unsubscribe`, a send to a closed channel may occur.

This is **expected** and internally handled via `recover`.

- The panic is caught
- Message is dropped
- Optional logging is triggered

---

### 2. Backpressure

Backpressure is implemented via Go channels:

- Slow subscriber → channel fills → affects publisher
- Behavior depends on delivery mode

---

### 3. No message persistence

- Messages are **not stored**
- Late subscribers do not receive past messages

---

### 4. Ordering

- Messages are delivered **in order per subscriber**
- No global ordering guarantees across subscribers

---

## Limitations

This library is intentionally simple and does NOT provide:

- Durable queues
- Message acknowledgment
- Exactly-once delivery
- Distributed communication
- Automatic scaling
- Subscriber isolation (fan-out is synchronous)

---

## When to Use

Good fit for:

- In-process event dispatching
- Simple pub/sub between goroutines
- Lightweight signaling
- Prototyping

Not suitable for:

- High-load messaging systems
- Distributed systems
- Critical guaranteed delivery pipelines

---

## License

MIT
