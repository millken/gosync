package gosync

import "sync"

type Map[K comparable, V any] struct {
	m sync.Map
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) { m.m.Delete(key) }

// Load loads the value stored in the map for a key, or nil if no value is present.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return value, ok
	}
	return v.(V), ok
}

// LoadOrStore loads the value stored in the map for a key, or stores and returns the given value if no value is present.
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return value, loaded
	}
	return v.(V), loaded
}

// LoadOrStore loads the value stored in the map for a key, or stores and returns the given value if no value is present.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	return a.(V), loaded
}

// Range calls f sequentially for each key and value present in the map. If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool { return f(key.(K), value.(V)) })
}

// Store stores the given value for a key.
func (m *Map[K, V]) Store(key K, value V) { m.m.Store(key, value) }
