package db

import (
	"fmt"
	"sync"
	"time"

	"apant_be/internal/domain"
)

type MemorySessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*domain.Session
	counter  int64
}

func NewMemorySessionRepository() *MemorySessionRepository {
	return &MemorySessionRepository{sessions: make(map[string]*domain.Session)}
}

func (r *MemorySessionRepository) Create() *domain.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	id := fmt.Sprintf("sess-%d", r.counter)
	now := time.Now()

	session := &domain.Session{ID: id, CreatedAt: now, UpdatedAt: now, Messages: []domain.Message{}}
	r.sessions[id] = session
	return cloneSession(session)
}

func (r *MemorySessionRepository) Get(id string) (*domain.Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[id]
	if !ok {
		return nil, false
	}

	return cloneSession(session), true
}

func (r *MemorySessionRepository) AddMessage(id string, msg domain.Message) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	msg.CreatedAt = time.Now()
	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	return cloneSession(session), nil
}

func (r *MemorySessionRepository) List() []*domain.Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*domain.Session, 0, len(r.sessions))
	for _, sess := range r.sessions {
		out = append(out, cloneSession(sess))
	}
	return out
}

func cloneSession(in *domain.Session) *domain.Session {
	if in == nil {
		return nil
	}

	out := &domain.Session{
		ID:        in.ID,
		CreatedAt: in.CreatedAt,
		UpdatedAt: in.UpdatedAt,
		Messages:  make([]domain.Message, len(in.Messages)),
	}
	copy(out.Messages, in.Messages)
	return out
}
