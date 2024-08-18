package gosync

import (
	"sync"
)

type PointerWithReset[T any] interface {
	*T

	Reset()
}

type Pool[T any, P PointerWithReset[T]] struct {
	pool sync.Pool
	New  func() P
}

func NewPool[T any, P PointerWithReset[T]](new func() P) *Pool[T, P] {
	return &Pool[T, P]{
		New: new,
	}
}

func (p *Pool[T, P]) Put(value P) {
	if value != nil {
		value.Reset()
		p.pool.Put(value)
	}
}

func (p *Pool[T, P]) Get() P {
	rv, ok := p.pool.Get().(P)
	if ok && rv != nil {
		return rv
	}

	return p.New()
}
