package control

import (
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	AppID     string    `json:"app_id,omitempty"`
	AppName   string    `json:"app_name,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	NodeName  string    `json:"node_name,omitempty"`
}

const subBufSize = 64

type subscriber struct {
	ch chan Event
	id int
}

type Ring struct {
	mu       sync.RWMutex
	events   []Event
	capacity int
	next     int
	seq      atomic.Int64
	subs     map[int]*subscriber
	subID    int
}

func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 256
	}
	return &Ring{
		events:   make([]Event, capacity),
		capacity: capacity,
		subs:     make(map[int]*subscriber),
	}
}

func (r *Ring) Push(e Event) {
	r.mu.Lock()
	e.ID = r.seq.Add(1)
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	r.events[r.next] = e
	r.next = (r.next + 1) % r.capacity
	for _, s := range r.subs {
		select {
		case s.ch <- e:
		default:
		}
	}
	r.mu.Unlock()
}

func (r *Ring) Recent(n int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n > r.capacity {
		n = r.capacity
	}
	if n < 1 {
		n = 1
	}
	start := r.next - n
	if start < 0 {
		start += r.capacity
	}
	res := make([]Event, 0, n)
	for i := 0; i < n; i++ {
		idx := (start + i) % r.capacity
		if idx < 0 {
			idx += r.capacity
		}
		e := r.events[idx]
		if e.ID == 0 {
			continue
		}
		res = append(res, e)
	}
	return res
}

func (r *Ring) Subscribe() (<-chan Event, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subID++
	id := r.subID
	ch := make(chan Event, subBufSize)
	r.subs[id] = &subscriber{ch: ch, id: id}
	return ch, func() {
		r.mu.Lock()
		delete(r.subs, id)
		r.mu.Unlock()
		close(ch)
	}
}

func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for i := range r.events {
		if r.events[i].ID != 0 {
			n++
		}
	}
	return n
}

func (r *Ring) Cap() int { return r.capacity }
