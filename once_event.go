package gosync

import (
	"sync"
	"sync/atomic"
)

/*
 *
 * Copyright 2018 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// This is a fork of google.golang.org/grpc/internal/grpcsync

// OnceEvent represents a one-time event that may occur in the future.
type OnceEvent struct {
	fired int32
	c     chan struct{}
	o     sync.Once
}

// Fire causes e to complete.  It is safe to call multiple times, and
// concurrently.  It returns true iff this call to Fire caused the signaling
// channel returned by Done to close.
func (e *OnceEvent) Fire() bool {
	ret := false
	e.o.Do(func() {
		atomic.StoreInt32(&e.fired, 1)
		close(e.c)
		ret = true
	})
	return ret
}

// Done returns a channel that will be closed when Fire is called.
func (e *OnceEvent) Done() <-chan struct{} {
	return e.c
}

// HasFired returns true if Fire has been called.
func (e *OnceEvent) HasFired() bool {
	return atomic.LoadInt32(&e.fired) == 1
}

// NewOnceEvent returns a new, ready-to-use Event.
func NewOnceEvent() *OnceEvent {
	return &OnceEvent{c: make(chan struct{})}
}
