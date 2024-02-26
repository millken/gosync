package gosync

import (
	"sync/atomic"
	"time"
)

// AtomicDuration is a wrapper with a simpler interface around atomic.(Add|Store|Load|CompareAndSwap)Int64 functions.
type AtomicDuration struct {
	int64
}

// NewAtomicDuration initializes a new AtomicDuration with a given value.
func NewAtomicDuration(duration time.Duration) AtomicDuration {
	return AtomicDuration{int64(duration)}
}

// Add atomically adds duration to the value.
func (d *AtomicDuration) Add(duration time.Duration) time.Duration {
	return time.Duration(atomic.AddInt64(&d.int64, int64(duration)))
}

// Set atomically sets duration as new value.
func (d *AtomicDuration) Set(duration time.Duration) {
	atomic.StoreInt64(&d.int64, int64(duration))
}

// Get atomically returns the current value.
func (d *AtomicDuration) Get() time.Duration {
	return time.Duration(atomic.LoadInt64(&d.int64))
}

// CompareAndSwap automatically swaps the old with the new value.
func (d *AtomicDuration) CompareAndSwap(oldval, newval time.Duration) (swapped bool) {
	return atomic.CompareAndSwapInt64(&d.int64, int64(oldval), int64(newval))
}
