package gosync

import (
	"runtime/debug"
	"sync"
	"testing"
)

type pooledValue[T any] struct {
	value T
}

func TestPoolNew(t *testing.T) {
	// Disable GC to avoid the victim cache during the test.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	p := NewPool(func() *pooledValue[string] {
		return &pooledValue[string]{
			value: "new",
		}
	})

	// Probabilistically, 75% of sync.Pool.Put calls will succeed when -race
	// is enabled (see ref below); attempt to make this quasi-deterministic by
	// brute force (i.e., put significantly more objects in the pool than we
	// will need for the test) in order to avoid testing without race enabled.
	//
	// ref: https://cs.opensource.google/go/go/+/refs/tags/go1.20.2:src/sync/pool.go;l=100-103
	for i := 0; i < 1_000; i++ {
		p.Put(&pooledValue[string]{
			value: t.Name(),
		})
	}

	// Ensure that we always get the expected value. Note that this must only
	// run a fraction of the number of times that Put is called above.
	for i := 0; i < 10; i++ {
		func() {
			x := p.Get()
			defer p.Put(x)
			if x.value != t.Name() {
				t.Fatalf("unexpected value: %s", x.value)
			}
		}()
	}

	// Depool all objects that might be in the pool to ensure that it's empty.
	for i := 0; i < 1_000; i++ {
		p.Get()
	}

	// Now that the pool is empty, it should use the value specified in the
	// underlying sync.Pool.New func.
	if x := p.Get(); x.value != "new" {
		t.Fatalf("unexpected value: %s", x.value)
	}
}

func TestPoolNewRace(t *testing.T) {
	p := NewPool(func() *pooledValue[int] {
		return &pooledValue[int]{
			value: -1,
		}
	})

	var wg sync.WaitGroup
	defer wg.Wait()

	// Run a number of goroutines that read and write pool object fields to
	// tease out races.
	for i := 0; i < 1_000; i++ {
		i := i

		wg.Add(1)
		go func() {
			defer wg.Done()

			x := p.Get()
			defer p.Put(x)

			// Must both read and write the field.
			if n := x.value; n >= -1 {
				x.value = i
			}
		}()
	}
}
