package gosync

import (
	"testing"
	"time"
)

type one int

func (o *one) Increment() {
	*o++
}

func run(t *testing.T, cache *Cache, o *one, c chan bool, i int) {
	cache.Do(time.Millisecond, func() { o.Increment() })
	if v := *o; int(v) != i {
		t.Errorf("cache failed inside run: %d is not %d", v, i)
	}
	c <- true
}

func TestCache(t *testing.T) {
	o := new(one)
	cache := new(Cache)
	c := make(chan bool)
	const N = 10
	for i := 0; i < N; i++ {
		go run(t, cache, o, c, i+1)
		time.Sleep(time.Millisecond * 10)
	}
	for i := 0; i < N; i++ {
		<-c
	}
	if *o != 10 {
		t.Errorf("cache failed outside run: %d is not 10", *o)
	}
}

func TestCacheWithZeroDuration(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Cache.Do did not panic")
		}
	}()
	o := new(one)
	cache := new(Cache)
	cache.Do(0, func() { o.Increment() })
}

func TestCachePanic(t *testing.T) {
	var cache Cache
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Cache.Do did not panic")
			}
		}()
		cache.Do(time.Second, func() {
			panic("failed")
		})
	}()

	cache.Do(time.Second, func() {
		t.Fatalf("Cache.Do called twice")
	})
}

func BenchmarkCache(b *testing.B) {
	var cache Cache
	f := func() {}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Do(time.Second, f)
		}
	})
}
