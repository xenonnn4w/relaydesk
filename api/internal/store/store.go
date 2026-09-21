package store

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"relaydesk/api/internal/domain"
)

var ErrNotFound = errors.New("ticket not found")

type TicketStore interface {
	List(context.Context) ([]domain.Ticket, error)
	Get(context.Context, string) (domain.Ticket, error)
	Create(context.Context, domain.Ticket) (domain.Ticket, error)
	Update(context.Context, domain.Ticket) (domain.Ticket, error)
}

type Memory struct {
	mu      sync.RWMutex
	tickets map[string]domain.Ticket
}

func NewMemory(seed []domain.Ticket) *Memory {
	m := &Memory{tickets: make(map[string]domain.Ticket)}
	for _, ticket := range seed {
		m.tickets[ticket.ID] = ticket
	}
	return m
}

func (m *Memory) List(_ context.Context) ([]domain.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Ticket, 0, len(m.tickets))
	for _, ticket := range m.tickets {
		out = append(out, ticket)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (m *Memory) Get(_ context.Context, id string) (domain.Ticket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ticket, ok := m.tickets[id]
	if !ok {
		return domain.Ticket{}, ErrNotFound
	}
	return ticket, nil
}

func (m *Memory) Create(_ context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets[ticket.ID] = ticket
	return ticket, nil
}

func (m *Memory) Update(_ context.Context, ticket domain.Ticket) (domain.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tickets[ticket.ID]; !ok {
		return domain.Ticket{}, ErrNotFound
	}
	ticket.UpdatedAt = time.Now().UTC()
	m.tickets[ticket.ID] = ticket
	return ticket, nil
}
