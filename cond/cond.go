//go:build !solution

package cond

// A Locker represents an object that can be locked and unlocked.
type Locker interface {
	Lock()
	Unlock()
}

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *sync.Mutex or *sync.RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.
type Cond struct {
	L     Locker
	mu    chan struct{}
	queue [](chan struct{})
}

// New returns a new Cond with Locker l.
func New(l Locker) *Cond {
	cond := &Cond{
		L:     l,
		mu:    make(chan struct{}, 1),
		queue: make([](chan struct{}), 0),
	}
	cond.mu <- struct{}{}
	return cond
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine. After later resuming execution,
// Wait locks c.L before returning. Unlike in other systems,
// Wait cannot return unless awoken by Broadcast or Signal.
//
// Because c.L is not locked when Wait first resumes, the caller
// typically cannot assume that the condition is true when
// Wait returns. Instead, the caller should Wait in a loop:
//
//	c.L.Lock()
//	for !condition() {
//	    c.Wait()
//	}
//	... make use of condition ...
//	c.L.Unlock()
func (c *Cond) Wait() {
	ch := make(chan struct{})

	<-c.mu
	c.queue = append(c.queue, ch)
	c.mu <- struct{}{}

	c.L.Unlock()
	<-ch
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
func (c *Cond) Signal() {
	<-c.mu
	if len(c.queue) == 0 {
		c.mu <- struct{}{}
		return
	}
	ch := c.queue[0]
	c.queue = c.queue[1:]
	c.mu <- struct{}{}

	close(ch)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.
func (c *Cond) Broadcast() {
	<-c.mu
	if len(c.queue) == 0 {
		c.mu <- struct{}{}
		return
	}
	q := c.queue
	c.queue = nil
	for _, ch := range c.queue {
		close(ch)
	}
	c.mu <- struct{}{}

	for _, ch := range q {
		close(ch)
	}
}
