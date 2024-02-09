package svc

type UserStore interface {
	GetUser(UserAuth) (*User, error)
	GetUserSessionCount(UserID) uint16
}

type SessionStore interface {
	Add(SessionID) error
	Remove(SessionID)
}
