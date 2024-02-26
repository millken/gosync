package gosync

import (
	"errors"
	"sync"
)

// Unbounded is an implementation of an unbounded buffer which does not use
// extra goroutines. This is typically used for passing updates from one entity
// to another within gRPC.
//
// All methods on this type are thread-safe and don't block on anything except
// the underlying mutex used for synchronization.
//
// Unbounded supports values of any type to be stored in it by using a channel
// of `any`. This means that a call to Put() incurs an extra memory allocation,
// and also that users need a type assertion while reading. For performance
// critical code paths, using Unbounded is strongly discouraged and defining a
// new type specific implementation of this buffer is preferred. See
// internal/transport/transport.go for an example of this.
type Unbounded[T any] struct {
	c       chan T
	closed  bool
	closing bool
	mu      sync.Mutex
	backlog []T
}

// NewUnbounded returns a new instance of Unbounded.
func NewUnbounded[T any]() *Unbounded[T] {
	return &Unbounded[T]{c: make(chan T, 1)}
}

var errBufferClosed = errors.New("Put called on closed buffer.Unbounded")

// Put adds t to the unbounded buffer.
func (b *Unbounded[T]) Put(t T) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closing {
		return errBufferClosed
	}
	if len(b.backlog) == 0 {
		select {
		case b.c <- t:
			return nil
		default:
		}
	}
	b.backlog = append(b.backlog, t)
	return nil
}

// Load sends the earliest buffered data, if any, onto the read channel returned
// by Get(). Users are expected to call this every time they successfully read a
// value from the read channel.
func (b *Unbounded[T]) Load() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.backlog) > 0 {
		select {
		case b.c <- b.backlog[0]:
			b.backlog = b.backlog[1:]
		default:
		}
	} else if b.closing && !b.closed {
		close(b.c)
	}
}

// Get returns a read channel on which values added to the buffer, via Put(),
// are sent on.
//
// Upon reading a value from this channel, users are expected to call Load() to
// send the next buffered value onto the channel if there is any.
//
// If the unbounded buffer is closed, the read channel returned by this method
// is closed after all data is drained.
func (b *Unbounded[T]) Get() <-chan T {
	return b.c
}

// Close closes the unbounded buffer. No subsequent data may be Put(), and the
// channel returned from Get() will be closed after all the data is read and
// Load() is called for the final time.
func (b *Unbounded[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closing {
		return
	}
	b.closing = true
	if len(b.backlog) == 0 {
		b.closed = true
		close(b.c)
	}
}
