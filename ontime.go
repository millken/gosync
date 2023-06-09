package gosync

import (
	"sync"
	"sync/atomic"
	"time"
)

// Ontime provides a mechanism to cache a value for a given duration.
type Ontime struct {
	done uint32
	m    sync.Mutex
}

// Do executes f if the cache is empty, and caches the result for duration.
func (o *Ontime) Do(duration time.Duration, f func()) {
	if duration == 0 {
		panic("duration must be non-zero")
	}
	if atomic.LoadUint32(&o.done) == 0 {
		defer func() {
			time.AfterFunc(duration, func() {
				if atomic.LoadUint32(&o.done) == 1 {
					atomic.StoreUint32(&o.done, 0)
				}
			})
		}()
		o.doSlow(f)
	}
}

func (o *Ontime) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
