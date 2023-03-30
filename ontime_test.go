package gosync

import (
	"sync/atomic"
	"testing"
	"time"
)

type one int32

func (o *one) Increment() {
	atomic.AddInt32((*int32)(o), 1)
}

func (o *one) Get() int32 {
	return atomic.LoadInt32((*int32)(o))
}

func run(t *testing.T, cache *Ontime, o *one, c chan bool, i int) {
	cache.Do(time.Millisecond, func() { o.Increment() })
	if v := o.Get(); int(v) != i {
		t.Errorf("cache failed inside run: %d is not %d", v, i)
	}
	c <- true
}

func TestOntime(t *testing.T) {
	o := new(one)
	cache := new(Ontime)
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

func TestOntimeWithZeroDuration(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Cache.Do did not panic")
		}
	}()
	o := new(one)
	a := new(Ontime)
	a.Do(0, func() { o.Increment() })
}

func TestOntimePanic(t *testing.T) {
	var on Ontime
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Cache.Do did not panic")
			}
		}()
		on.Do(time.Second, func() {
			panic("failed")
		})
	}()

	on.Do(time.Second, func() {
		t.Fatalf("Cache.Do called twice")
	})
}

func BenchmarkCache(b *testing.B) {
	var on Ontime
	f := func() {}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			on.Do(time.Second, f)
		}
	})
}
