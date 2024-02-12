package svc

import (
	"io"

	"github.com/quic-go/quic-go"
)

type EdgeID uint64
type RelayID uint64
type SessionID uint64

type Destination string
type Region string

type Conn interface {
	io.Reader
	io.Writer
	Close() error
}

type SessionReaderWriter interface {
	Conn
	ID() SessionID
}

type EdgeConn interface {
	Conn
	ID() EdgeID
}

type GatewayAuthHandleFunc func(string, string) (*User, error)
type GatewayRelayRegisterHandleFunc func(quic.Connection) (RelayID, <-chan error, error)
type GatewayRelayRegionsHandleFunc func(RelayID, map[string]string) error
type GatewaySessionHandleFunc func(*User, Destination, Region, Conn) error

type RelayGatewayHandleFunc func(quic.Stream) error
type RelayClientHandleFunc func(EdgeConn) error

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
