//go:build go1.18
// +build go1.18

// inspired by https://github.com/a8m/syncmap, but it is not generic and requires auto-generation, so I rewrote it.
package gosync

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type entry[V any] struct {
	p        atomic.Pointer[V]
	expunged unsafe.Pointer
}

func newEntry[V any](i V, expunged unsafe.Pointer) *entry[V] {
	e := &entry[V]{
		expunged: expunged,
	}
	e.p.Store(&i)
	return e
}

func (e *entry[V]) load() (value V, ok bool) {
	p := e.p.Load()
	if p == nil || p == (*V)(e.expunged) {
		return value, false
	}
	return *p, true
}

func (e *entry[V]) tryStore(v *V) bool {
	for {
		p := e.p.Load()
		if p == (*V)(e.expunged) {
			return false
		}
		if e.p.CompareAndSwap(p, v) {
			return true
		}
	}
}

func (e *entry[V]) unexpungeLocked() (wasExpunged bool) {
	return e.p.Swap((*V)(e.expunged)) == (*V)(e.expunged)
}

func (e *entry[V]) storeLocked(v *V) {
	e.p.Store(v)
}

// tryLoadOrStore atomically loads or stores a value if the entry is not
// expunged.
//
// If the entry is expunged, tryLoadOrStore leaves the entry unchanged and
// returns with ok==false.
func (e *entry[V]) tryLoadOrStore(v V) (actual V, loaded, ok bool) {
	p := e.p.Load()
	if p == (*V)(e.expunged) {
		return actual, false, false
	}
	if p != nil {
		return *p, true, true
	}

	// Copy the interface after the first load to make this method more amenable
	// to escape analysis: if we hit the "load" path or the entry is expunged, we
	// shouldn't bother heap-allocating.
	ic := v
	for {
		if e.p.CompareAndSwap(nil, &ic) {
			return v, false, true
		}
		p = e.p.Load()
		if p == (*V)(e.expunged) {
			return actual, false, false
		}
		if p != nil {
			return *p, true, true
		}
	}
}

func (e *entry[V]) delete() (value V, ok bool) {
	for {
		p := e.p.Load()
		if p == nil || p == (*V)(e.expunged) {
			return value, false
		}
		if e.p.CompareAndSwap(p, nil) {
			return *p, true
		}
	}
}

func (e *entry[V]) tryExpungeLocked() (isExpunged bool) {
	p := e.p.Load()
	for p == nil {
		if e.p.CompareAndSwap(nil, (*V)(e.expunged)) {
			return true
		}
		p = e.p.Load()
	}
	return p == (*V)(e.expunged)
}

// readOnly is an immutable struct stored atomically in the Map.read field.
type readOnly[K comparable, V any] struct {
	m       map[K]*entry[V]
	amended bool // true if the dirty map contains some key not in m.
}

type Map[K comparable, V any] struct {
	mu       sync.Mutex
	read     atomic.Value
	dirty    map[K]*entry[V]
	misses   int
	expunged unsafe.Pointer
}

func NewMap[K comparable, V any]() *Map[K, V] {
	m := &Map[K, V]{
		read:     atomic.Value{},
		expunged: unsafe.Pointer(new(V)),
	}
	return m
}

// Load loads the value stored in the map for a key, or nil if no value is present.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	read := m.loadReadOnly()
	e, ok := read.m[key] //todo: performance
	if !ok && read.amended {
		m.mu.Lock()
		// Avoid reporting a spurious miss if m.dirty got promoted while we were
		// blocked on m.mu. (If further loads of the same key will not miss, it's
		// not worth copying the dirty map for this key.)
		read = m.loadReadOnly()
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			// Regardless of whether the entry was present, record a miss: this key
			// will take the slow path until the dirty map is promoted to the read
			// map.
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if !ok {
		return value, false
	}
	return e.load()
}

func (m *Map[K, V]) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}
	m.read.Store(&readOnly[K, V]{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

func (m *Map[K, V]) loadReadOnly() readOnly[K, V] {
	if p := m.read.Load(); p != nil {
		return *p.(*readOnly[K, V])
	}
	return readOnly[K, V]{}
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	read := m.loadReadOnly()
	if e, ok := read.m[key]; ok && e.tryStore(&value) {
		return
	}

	m.mu.Lock()
	read = m.loadReadOnly()
	if e, ok := read.m[key]; ok {
		if e.unexpungeLocked() {
			// The entry was previously expunged, which implies that there is a
			// non-nil dirty map and this entry is not in it.
			m.dirty[key] = e
		}
		e.storeLocked(&value)
	} else if e, ok := m.dirty[key]; ok {
		e.storeLocked(&value)
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			m.dirtyLocked()
			m.read.Store(&readOnly[K, V]{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value, m.expunged)
	}
	m.mu.Unlock()
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	// Avoid locking if it's a clean hit.
	read := m.loadReadOnly()
	if e, ok := read.m[key]; ok {
		actual, loaded, ok := e.tryLoadOrStore(value)
		if ok {
			return actual, loaded
		}
	}

	m.mu.Lock()
	read = m.loadReadOnly()
	if e, ok := read.m[key]; ok {
		if e.unexpungeLocked() {
			m.dirty[key] = e
		}
		actual, loaded, _ = e.tryLoadOrStore(value)
	} else if e, ok := m.dirty[key]; ok {
		actual, loaded, _ = e.tryLoadOrStore(value)
		m.missLocked()
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			m.dirtyLocked()
			m.read.Store(&readOnly[K, V]{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value, m.expunged)
		actual, loaded = value, false
	}
	m.mu.Unlock()

	return actual, loaded
}

func (m *Map[K, V]) dirtyLocked() {
	if m.dirty != nil {
		return
	}

	read := m.loadReadOnly()
	m.dirty = make(map[K]*entry[V], len(read.m))
	for k, e := range read.m {
		if !e.tryExpungeLocked() {
			m.dirty[k] = e
		}
	}
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	read := m.loadReadOnly()
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()
		read = m.loadReadOnly()
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			delete(m.dirty, key)
			// Regardless of whether the entry was present, record a miss: this key
			// will take the slow path until the dirty map is promoted to the read
			// map.
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if ok {
		return e.delete()
	}
	return value, false
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.LoadAndDelete(key)
}

// Range calls f sequentially for each key and value present in the map. If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	// We need to be able to iterate over all of the keys that were already
	// present at the start of the call to Range.
	// If read.amended is false, then read.m satisfies that property without
	// requiring us to hold m.mu for a long time.
	read := m.loadReadOnly()
	if read.amended {
		// m.dirty contains keys not in read.m. Fortunately, Range is already O(N)
		// (assuming the caller does not break out early), so a call to Range
		// amortizes an entire copy of the map: we can promote the dirty copy
		// immediately!
		m.mu.Lock()
		read = m.loadReadOnly()
		if read.amended {
			read = readOnly[K, V]{m: m.dirty}
			m.read.Store(&read)
			m.dirty = nil
			m.misses = 0
		}
		m.mu.Unlock()
	}

	for k, e := range read.m {
		v, ok := e.load()
		if !ok {
			continue
		}
		if !f(k, v) {
			break
		}
	}
}
