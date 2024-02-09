package store

import (
	"errors"

	"github.com/bacv/kingip/svc"
)

// MockUserStore is a mock implementation of UserStore for testing purposes.
type MockUserStore struct {
	Users         map[svc.UserAuth]*svc.User
	SessionCounts map[svc.UserID]uint16
}

func NewMockUserStore() *MockUserStore {
	return &MockUserStore{
		Users:         make(map[svc.UserAuth]*svc.User),
		SessionCounts: make(map[svc.UserID]uint16),
	}
}

func (store *MockUserStore) GetUser(auth svc.UserAuth) (*svc.User, error) {
	if user, exists := store.Users[auth]; exists {
		return user, nil
	}
	return nil, errors.New("user not found")
}

func (store *MockUserStore) GetUserSessionCount(userID svc.UserID) uint16 {
	if count, exists := store.SessionCounts[userID]; exists {
		return count
	}
	return 0
}
