package subpub

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

type MessageHandler func(msg interface{})

type Subscription interface {
	Unsubscribe()
}

type SubPub interface {
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	Publish(subject string, msg interface{}) error
	Close(ctx context.Context) error
}

type subscription struct {
	subject string
	id      string
	handler MessageHandler
	subPub  *subPubImpl
}

func (s *subscription) Unsubscribe() {
	s.subPub.unsubscribe(s.subject, s.id)
}

type subscriber struct {
	id      string
	handler MessageHandler
	msgCh   chan interface{}
}

type subPubImpl struct {
	mu           sync.RWMutex
	subjects     map[string]map[string]*subscriber
	wg           sync.WaitGroup
	closed       bool
	closeCh      chan struct{}
	subscriberID int
}

func NewSubPub() SubPub {
	sp := &subPubImpl{
		subjects:     make(map[string]map[string]*subscriber),
		closeCh:      make(chan struct{}),
		subscriberID: 0,
	}
	return sp
}

func (sp *subPubImpl) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.closed {
		return nil, errors.New("subpub is closed")
	}

	if _, exists := sp.subjects[subject]; !exists {
		sp.subjects[subject] = make(map[string]*subscriber)
	}

	sp.subscriberID++
	id := strconv.Itoa(sp.subscriberID)

	sub := &subscriber{
		id:      id,
		handler: cb,
		msgCh:   make(chan interface{}, 100),
	}

	sp.subjects[subject][id] = sub

	sp.wg.Add(1)
	go sp.handleMessages(sub)

	return &subscription{
		subject: subject,
		id:      id,
		handler: cb,
		subPub:  sp,
	}, nil
}

func (sp *subPubImpl) Publish(subject string, msg interface{}) error {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	subscribers, exists := sp.subjects[subject]
	if !exists || len(subscribers) == 0 {
		return nil
	}

	for _, sub := range subscribers {
		select {
		case sub.msgCh <- msg:
		default:
		}
	}

	return nil
}

func (sp *subPubImpl) Close(ctx context.Context) error {
	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		return nil
	}
	sp.closed = true
	sp.mu.Unlock()

	close(sp.closeCh)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	select {
	case <-time.After(100 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (sp *subPubImpl) unsubscribe(subject, id string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if subscribers, exists := sp.subjects[subject]; exists {
		if sub, ok := subscribers[id]; ok {
			close(sub.msgCh)
			delete(subscribers, id)
		}
		if len(subscribers) == 0 {
			delete(sp.subjects, subject)
		}
	}
}

func (sp *subPubImpl) handleMessages(sub *subscriber) {
	defer sp.wg.Done()

	for {
		select {
		case msg, ok := <-sub.msgCh:
			if !ok {
				return
			}
			sub.handler(msg)
		case <-sp.closeCh:
		}
	}
}
