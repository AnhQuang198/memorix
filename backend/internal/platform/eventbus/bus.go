package eventbus

import (
	"context"
	"sync"
)

// Event là domain event (tên PascalCase quá khứ, vd CardGraded).
type Event struct {
	Name    string
	Payload any
}

type Handler func(context.Context, Event)

// Bus là port; MVP dùng InProcess fire-and-forget (AD-8).
// Interface sẵn để nâng transactional outbox ở V1.
type Bus interface {
	Publish(ctx context.Context, e Event)
	Subscribe(name string, h Handler)
}

type InProcess struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	wg       sync.WaitGroup
}

func NewInProcess() *InProcess {
	return &InProcess{handlers: map[string][]Handler{}}
}

func (b *InProcess) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

func (b *InProcess) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	hs := b.handlers[e.Name]
	b.mu.RUnlock()
	for _, h := range hs {
		b.wg.Add(1)
		h := h
		go func() {
			defer b.wg.Done()
			h(ctx, e)
		}()
	}
}

// Wait chờ mọi handler async xong (dùng trong test/shutdown).
func (b *InProcess) Wait() { b.wg.Wait() }
