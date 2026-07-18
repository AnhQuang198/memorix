package eventbus

import (
	"context"
	"sync"
	"testing"
)

func TestInProcessBus_PublishDelivers(t *testing.T) {
	bus := NewInProcess()
	var mu sync.Mutex
	got := []string{}
	bus.Subscribe("CardGraded", func(_ context.Context, e Event) {
		mu.Lock()
		got = append(got, e.Name)
		mu.Unlock()
	})
	bus.Publish(context.Background(), Event{Name: "CardGraded", Payload: nil})
	bus.Wait()
	if len(got) != 1 || got[0] != "CardGraded" {
		t.Errorf("handler not called correctly: %v", got)
	}
}

func TestInProcessBus_IgnoresUnsubscribed(t *testing.T) {
	bus := NewInProcess()
	called := false
	bus.Subscribe("A", func(context.Context, Event) { called = true })
	bus.Publish(context.Background(), Event{Name: "B"})
	bus.Wait()
	if called {
		t.Error("handler for A should not fire on B")
	}
}
