package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRingRetainsMostRecentEvents(t *testing.T) {
	r := NewRing(3)
	for _, message := range []string{"one", "two", "three", "four"} {
		r.Push(Event{Type: "test", Message: message})
	}

	got := r.Recent(10)
	if len(got) != 3 {
		t.Fatalf("Recent() returned %d events, want 3", len(got))
	}
	for i, want := range []string{"two", "three", "four"} {
		if got[i].Message != want {
			t.Fatalf("event %d message = %q, want %q", i, got[i].Message, want)
		}
		if got[i].ID != int64(i+2) {
			t.Fatalf("event %d ID = %d, want %d", i, got[i].ID, i+2)
		}
		if got[i].Timestamp.IsZero() {
			t.Fatalf("event %d has zero timestamp", i)
		}
	}
}

func TestRingSubscriptionAndCancel(t *testing.T) {
	r := NewRing(2)
	ch, cancel := r.Subscribe()
	r.Push(Event{Message: "first"})

	select {
	case got := <-ch:
		if got.Message != "first" {
			t.Fatalf("message = %q, want first", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event")
	}

	cancel()
	r.Push(Event{Message: "second"})
	if _, ok := <-ch; ok {
		t.Fatal("subscription channel remained open after cancel")
	}
}

func TestRingConcurrentPushAndCancel(t *testing.T) {
	r := NewRing(8)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		ch, cancel := r.Subscribe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			cancel()
		}()
		r.Push(Event{Message: "event"})
		_ = ch
	}
	wg.Wait()
}

func TestActivityReturnsRecentEvents(t *testing.T) {
	s := &Server{events: NewRing(4)}
	s.events.Push(Event{Message: "one"})
	s.events.Push(Event{Message: "two"})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/activity?limit=1", nil)
	w := httptest.NewRecorder()

	s.activity(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []Event
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "two" {
		t.Fatalf("events = %#v, want latest event", got)
	}
}
