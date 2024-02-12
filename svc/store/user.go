package store

import (
	"errors"
	"sync"

	"github.com/bacv/kingip/svc"
)

// MockUserStore is a mock implementation of UserStore for testing purposes.
type MockUserStore struct {
	mu            sync.Mutex
	Users         map[svc.UserAuth]*svc.User
	SessionCounts map[svc.UserID]uint16
	TotalUsedMBs  map[svc.UserID]float64
}

func NewMockUserStore() *MockUserStore {
	return &MockUserStore{
		Users:         make(map[svc.UserAuth]*svc.User),
		SessionCounts: make(map[svc.UserID]uint16),
		TotalUsedMBs:  make(map[svc.UserID]float64),
	}
}

func (store *MockUserStore) GetUser(auth svc.UserAuth) (*svc.User, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if user, exists := store.Users[auth]; exists {
		return user, nil
	}
	return nil, errors.New("user not found")
}

func (store *MockUserStore) GetUserSessionCount(userID svc.UserID) uint16 {
	store.mu.Lock()
	defer store.mu.Unlock()

	if count, exists := store.SessionCounts[userID]; exists {
		return count
	}
	return 0
}

func (store *MockUserStore) UpdateUserTotalUsedMBs(userID svc.UserID, mbs float64) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.TotalUsedMBs[userID] += mbs
}

func (store *MockUserStore) GetUserTotalUsedMBs(userID svc.UserID) float64 {
	store.mu.Lock()
	defer store.mu.Unlock()

	if totalMBs, exists := store.TotalUsedMBs[userID]; exists {
		return totalMBs
	}
	return 0
}
