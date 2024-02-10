package store

import (
	"sync"

	"github.com/bacv/kingip/svc"
)

type MockSessionStore struct {
	mu            sync.Mutex
	SessionCounts map[svc.UserID]uint16
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		SessionCounts: make(map[svc.UserID]uint16),
	}
}

func (store *MockSessionStore) SessionAdd(userID svc.UserID) uint16 {
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.SessionCounts[userID]; exists {
		store.SessionCounts[userID]++
	} else {
		store.SessionCounts[userID] = 1
	}
	return store.SessionCounts[userID]
}

func (store *MockSessionStore) SessionRemove(userID svc.UserID) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if count, exists := store.SessionCounts[userID]; exists && count > 0 {
		store.SessionCounts[userID]--
	}
}
