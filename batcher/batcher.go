//go:build !solution

package batcher

import (
	"sync"

	"gitlab.com/slon/shad-go/batcher/slow"
)

type result struct {
	done  chan struct{}
	value interface{}
}

type Batcher struct {
	mu      sync.Mutex
	v       *slow.Value
	version int
	values  map[int]*result
}

func NewBatcher(v *slow.Value) *Batcher {
	return &Batcher{
		v:      v,
		values: make(map[int]*result),
	}
}

func (b *Batcher) load(version int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	val := b.v.Load()
	b.values[version].value = val
	close(b.values[version].done)
	delete(b.values, version)
}

func (b *Batcher) Load() interface{} {
	b.mu.Lock()

	if _, ok := b.values[b.version]; !ok {
		b.values[b.version] = &result{done: make(chan struct{})}
		go b.load(b.version)
	}

	res := b.values[b.version]
	b.mu.Unlock()

	<-res.done

	return res.value
}

func (b *Batcher) Store(v interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.version++
	b.v.Store(v)
}
