package svc

import "time"

type UserID uint64

type UserConfig struct {
	maxSessions        uint16
	maxSessionDuration time.Duration
	maxGBs             float64
}

func NewUserConfig(sessions uint16, gbs float64, duration time.Duration) UserConfig {
	return UserConfig{
		maxSessions:        sessions,
		maxGBs:             gbs,
		maxSessionDuration: duration,
	}
}

func DefaultUserConfig() UserConfig {
	return NewUserConfig(10, 1, time.Hour)
}

type User struct {
	name   string
	id     UserID
	config UserConfig
}

func NewUser(name string, id UserID, config UserConfig) *User {
	return &User{
		name:   name,
		id:     id,
		config: config,
	}
}

func (u *User) Name() string {
	return u.name
}

func (u *User) ID() UserID {
	return u.id
}

func (u *User) MaxSessions() uint16 {
	return u.config.maxSessions
}

func (u *User) MaxGBs() float64 {
	return float64(u.config.maxGBs)
}

func (u *User) MaxSessionDuration() time.Duration {
	return u.config.maxSessionDuration
}

type UserAuth struct {
	Name     string
	Password string
}
