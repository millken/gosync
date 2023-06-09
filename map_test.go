package gosync

import (
	"math/rand"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// isNil gets whether the object is nil or not.
func isNil(object interface{}) bool {
	if object == nil {
		return true
	}
	value := reflect.ValueOf(object)
	kind := value.Kind()
	if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
		return true
	}
	return false
}

// areEqual gets whether a equals b or not.
func areEqual(a, b interface{}) bool {
	if isNil(a) && isNil(b) {
		return true
	}
	if isNil(a) || isNil(b) {
		return false
	}
	if reflect.DeepEqual(a, b) {
		return true
	}
	aValue := reflect.ValueOf(a)
	bValue := reflect.ValueOf(b)
	return aValue == bValue
}

type mapInterface interface {
	Load(any) (any, bool)
	Store(key, value any)
	LoadOrStore(key, value any) (actual any, loaded bool)
	LoadAndDelete(key any) (value any, loaded bool)
	Delete(any)
	Range(func(key, value any) (shouldContinue bool))
}

func TestConcurrentRange(t *testing.T) {
	const mapSize = 1 << 10

	var m Map[int64, int64]
	for n := int64(1); n <= mapSize; n++ {
		m.Store(n, int64(n))
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		close(done)
		wg.Wait()
	}()
	for g := int64(runtime.GOMAXPROCS(0)); g > 0; g-- {
		r := rand.New(rand.NewSource(g))
		wg.Add(1)
		go func(g int64) {
			defer wg.Done()
			for i := int64(0); ; i++ {
				select {
				case <-done:
					return
				default:
				}
				for n := int64(1); n < mapSize; n++ {
					if r.Int63n(mapSize) == 0 {
						m.Store(n, n*i*g)
					} else {
						m.Load(n)
					}
				}
			}
		}(g)
	}

	iters := 1 << 10
	if testing.Short() {
		iters = 16
	}
	for n := iters; n > 0; n-- {
		seen := make(map[int64]bool, mapSize)

		m.Range(func(ki, vi int64) bool {
			k, v := ki, vi
			if v%k != 0 {
				t.Fatalf("while Storing multiples of %v, Range saw value %v", k, v)
			}
			if seen[k] {
				t.Fatalf("Range visited key %v twice", k)
			}
			seen[k] = true
			return true
		})

		if len(seen) != mapSize {
			t.Fatalf("Range visited %v elements of %v-element Map", len(seen), mapSize)
		}
	}
}

func TestIssue40999(t *testing.T) {
	var m Map[*int, struct{}]

	// Since the miss-counting in missLocked (via Delete)
	// compares the miss count with len(m.dirty),
	// add an initial entry to bias len(m.dirty) above the miss count.
	m.Store(nil, struct{}{})

	var finalized uint32

	// Set finalizers that count for collected keys. A non-zero count
	// indicates that keys have not been leaked.
	for atomic.LoadUint32(&finalized) == 0 {
		p := new(int)
		runtime.SetFinalizer(p, func(*int) {
			atomic.AddUint32(&finalized, 1)
		})
		m.Store(p, struct{}{})
		m.Delete(p)
		runtime.GC()
	}
}

func TestMapRangeNestedCall(t *testing.T) { // Issue 46399
	var m Map[int, string]
	for i, v := range [3]string{"hello", "world", "Go"} {
		m.Store(i, v)
	}
	m.Range(func(key int, value string) bool {
		m.Range(func(key int, value string) bool {
			// We should be able to load the key offered in the Range callback,
			// because there are no concurrent Delete involved in this tested map.
			if v, ok := m.Load(key); !ok || !reflect.DeepEqual(v, value) {
				t.Fatalf("Nested Range loads unexpected value, got %+v want %+v", v, value)
			}

			// We didn't keep 42 and a value into the map before, if somehow we loaded
			// a value from such a key, meaning there must be an internal bug regarding
			// nested range in the Map.
			if _, loaded := m.LoadOrStore(42, "dummy"); loaded {
				t.Fatalf("Nested Range loads unexpected value, want store a new value")
			}

			// Try to Store then LoadAndDelete the corresponding value with the key
			// 42 to the Map. In this case, the key 42 and associated value should be
			// removed from the Map. Therefore any future range won't observe key 42
			// as we checked in above.
			val := "sync.Map"
			m.Store(42, val)
			if v, loaded := m.LoadAndDelete(42); !loaded || !reflect.DeepEqual(v, val) {
				t.Fatalf("Nested Range loads unexpected value, got %v, want %v", v, val)
			}
			return true
		})

		// Remove key from Map on-the-fly.
		m.Delete(key)
		return true
	})

	// After a Range of Delete, all keys should be removed and any
	// further Range won't invoke the callback. Hence length remains 0.
	length := 0
	m.Range(func(key int, value string) bool {
		length++
		return true
	})

	if length != 0 {
		t.Fatalf("Unexpected sync.Map size, got %v want %v", length, 0)
	}
}

func TestIntMap(t *testing.T) {
	m := NewMap[int, int]()
	m.Store(1, 2)
	_, ok := m.Load(1)
	if !ok {
		t.Fatal("value should be existed")
	}
	m.Delete(1)
	_, ok = m.Load(1)
	if ok {
		t.Fatal("value should not be existed")
	}
	r, loaded := m.LoadOrStore(1, 2)
	if loaded {
		t.Fatal("value should not be loaded")
	}
	lr, loaded := m.LoadOrStore(1, r)
	if !loaded {
		t.Fatal("value should not be loaded")
	}
	if lr != r {
		t.Fatal("loaded value should be the same")
	}
	s, _ := m.LoadOrStore(2, 3)
	if m.Len() != 2 {
		t.Fatalf("length should be 2, got %d", m.Len())
	}
	m2 := m.Clone()
	if areEqual(m, m2) {
		t.Fatal("clone should be the same")
	}
	m2.Clear()
	if m2.Len() != 0 {
		t.Fatalf("length should be 0, got %d", m2.Len())
	}
	kv := map[int]int{1: r, 2: s}
	m.Range(func(key, value int) bool {
		v, ok := kv[key]
		if !ok {
			t.Fatal("keys do not match")
		}
		if value != v {
			t.Fatal("values do not match")
		}
		delete(kv, key)
		return true
	})
}

func TestRequests(t *testing.T) {
	m := NewMap[string, *http.Request]()
	m.Store("r", &http.Request{})
	_, ok := m.Load("r")
	if !ok {
		t.Fatal("value should be existed")
	}
	v, ok := m.LoadAndDelete("r")
	if !ok || v == nil {
		t.Fatal("value should be existed")
	}
	_, ok = m.Load("r")
	if ok {
		t.Fatal("value should not be existed")
	}
	r, loaded := m.LoadOrStore("r", &http.Request{})
	if loaded {
		t.Fatal("value should not be loaded")
	}
	lr, loaded := m.LoadOrStore("r", r)
	if !loaded {
		t.Fatal("value should not be loaded")
	}
	if lr != r {
		t.Fatal("loaded value should be the same")
	}
	s, _ := m.LoadOrStore("s", &http.Request{})
	kv := map[string]*http.Request{"r": r, "s": s}
	m.Range(func(key string, value *http.Request) bool {
		v, ok := kv[key]
		if !ok {
			t.Fatal("keys do not match")
		}
		if value != v {
			t.Fatal("values do not match")
		}
		delete(kv, key)
		return true
	})
}
