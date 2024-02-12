package svc

type UserStore interface {
	GetUser(UserAuth) (*User, error)
	GetUserSessionCount(UserID) uint16
}

type SessionStore interface {
	SessionAdd(UserID) uint16
	SessionRemove(UserID)
}

type BandwidthStore interface {
	UpdateUserTotalUsedMBs(UserID, float64)
	GetUserTotalUsedMBs(UserID) float64
}
