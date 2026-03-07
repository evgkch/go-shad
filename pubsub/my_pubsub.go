//go:build !solution

package pubsub

import (
	"context"
	"errors"
	"sync"
)

var ErrClosed = errors.New("pubsub: closed")

var _ Subscription = (*MySubscription)(nil)

type MySubscription struct {
	subj   string
	cb     MsgHandler
	p      *MyPubSub
	mu     sync.Mutex
	queue  []interface{}
	notify chan struct{}
	done   chan struct{}
}

func (s *MySubscription) push(msg interface{}) {
	s.mu.Lock()
	s.queue = append(s.queue, msg)
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *MySubscription) Unsubscribe() {
	s.p.mu.Lock()
	messages := s.p.messages[s.subj]
	for i, v := range messages {
		if v == s {
			messages[i] = messages[len(messages)-1]
			s.p.messages[s.subj] = messages[:len(messages)-1]
			break
		}
	}
	s.p.mu.Unlock()

	close(s.notify)
	<-s.done
}

var _ PubSub = (*MyPubSub)(nil)

type MyPubSub struct {
	err      error
	closed   chan struct{}
	closeOne sync.Once
	mu       sync.Mutex
	messages map[string][]*MySubscription
}

func NewPubSub() PubSub {
	return &MyPubSub{
		closed:   make(chan struct{}),
		messages: make(map[string][]*MySubscription),
	}
}

func (p *MyPubSub) Subscribe(subj string, cb MsgHandler) (Subscription, error) {
	select {
	case <-p.closed:
		return nil, p.err
	default:
		p.mu.Lock()
		defer p.mu.Unlock()
		select {
		case <-p.closed:
			return nil, p.err
		default:
		}
		s := &MySubscription{
			subj:   subj,
			cb:     cb,
			p:      p,
			notify: make(chan struct{}, 1),
			done:   make(chan struct{}),
		}
		go func() {
			defer close(s.done)
			for range s.notify {
				for {
					s.mu.Lock()
					if len(s.queue) == 0 {
						s.mu.Unlock()
						break
					}
					msg := s.queue[0]
					s.queue = s.queue[1:]
					s.mu.Unlock()
					s.cb(msg)
				}
			}
		}()
		p.messages[subj] = append(p.messages[subj], s)
		return s, nil
	}
}

func (p *MyPubSub) Publish(subj string, msg interface{}) error {
	select {
	case <-p.closed:
		return p.err
	default:
		p.mu.Lock()
		subs := make([]*MySubscription, len(p.messages[subj]))
		copy(subs, p.messages[subj])
		p.mu.Unlock()

		for _, s := range subs {
			s.push(msg)
		}
		return nil
	}
}

func (p *MyPubSub) Close(ctx context.Context) error {
	p.closeOne.Do(func() {
		p.mu.Lock()
		p.err = ErrClosed
		close(p.closed)
		subs := p.messages
		p.messages = make(map[string][]*MySubscription)
		p.mu.Unlock()

		for _, list := range subs {
			for _, s := range list {
				close(s.notify)
			}
		}

		for _, list := range subs {
			for _, s := range list {
				select {
				case <-ctx.Done():
					return
				case <-s.done:
				}
			}
		}
	})
	return ctx.Err()
}
