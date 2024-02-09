package svc

type UserID uint64

type UserConfig struct {
	maxSessions uint16
}

func DefaultUserConfig() UserConfig {
	return UserConfig{
		maxSessions: 10,
	}
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

type UserAuth struct {
	Name     string
	Password string
}
