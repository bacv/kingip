package svc

import (
	"io"

	"github.com/quic-go/quic-go"
)

type Conn interface {
	io.Reader
	io.Writer
	Close() error
}

type SessionID uint64

type SessionReaderWriter interface {
	io.Writer
	io.Reader
	ID() SessionID
}

type Destination string
type Region string

type GatewayAuthHandleFunc func(string, string) error
type GatewayRelayRegisterHandleFunc func(quic.Connection) (RelayID, <-chan error, error)
type GatewayRelayRegionsHandleFunc func(RelayID, map[string]string) error
type GatewaySessionHandleFunc func(Destination, Region, Conn) error
