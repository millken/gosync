package gosync

import "testing"

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
