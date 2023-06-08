package gosync

import "testing"

func TestAtomicInt32(t *testing.T) {
	var i AtomicInt32
	i.Add(1)
	if i.Get() != 1 {
		t.FailNow()
	}
	i.Add(-1)
	if i.Get() != 0 {
		t.FailNow()
	}
	i.Set(1)
	if i.Get() != 1 {
		t.FailNow()
	}
	swap := i.CompareAndSwap(1, 2)
	if !swap {
		t.FailNow()
	}
	if i.Get() != 2 {
		t.FailNow()
	}
}

func TestAtomicInt64(t *testing.T) {
	var i AtomicInt64
	i.Add(1)
	if i.Get() != 1 {
		t.FailNow()
	}
	i.Add(-1)
	if i.Get() != 0 {
		t.FailNow()
	}
	i.Set(1)
	if i.Get() != 1 {
		t.FailNow()
	}
	swap := i.CompareAndSwap(1, 2)
	if !swap {
		t.FailNow()
	}
	if i.Get() != 2 {
		t.FailNow()
	}
}

func TestAtomicBool(t *testing.T) {
	var i AtomicBool
	i.Set(true)
	if !i.Get() {
		t.FailNow()
	}
	i.Set(false)
	if i.Get() {
		t.FailNow()
	}
	swap := i.CompareAndSwap(false, true)
	if !swap {
		t.FailNow()
	}
	if !i.Get() {
		t.FailNow()
	}
}

func TestAtomicDuration(t *testing.T) {
	var i AtomicDuration
	i.Set(1)
	if i.Get() != 1 {
		t.FailNow()
	}
	i.Set(2)
	if i.Get() != 2 {
		t.FailNow()
	}
	swap := i.CompareAndSwap(2, 3)
	if !swap {
		t.FailNow()
	}
	if i.Get() != 3 {
		t.FailNow()
	}
}
