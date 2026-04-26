package domain

import (
	"sync"
	"time"
)

// DomainEvent represents an event that occurred in the domain.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() string
}

// BaseEvent provides a base implementation of DomainEvent.
type BaseEvent struct {
	Name        string    `json:"name"`
	Timestamp   time.Time `json:"occurred_at"`
	AggregateId string    `json:"aggregate_id"`
}

func (e BaseEvent) EventName() string     { return e.Name }
func (e BaseEvent) OccurredAt() time.Time { return e.Timestamp }
func (e BaseEvent) AggregateID() string   { return e.AggregateId }

// EventHandler is a function that handles a domain event.
type EventHandler func(event DomainEvent) error

// EventBus defines the interface for publishing and subscribing to domain events.
type EventBus interface {
	Publish(event DomainEvent) error
	Subscribe(eventName string, handler EventHandler)
}

// InMemoryEventBus is an in-memory implementation of EventBus.
type InMemoryEventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

// NewInMemoryEventBus creates a new InMemoryEventBus.
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// Publish sends an event to all registered handlers.
func (b *InMemoryEventBus) Publish(event DomainEvent) error {
	b.mu.RLock()
	handlers := b.handlers[event.EventName()]
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(event); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe registers a handler for a specific event name.
func (b *InMemoryEventBus) Subscribe(eventName string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
}
