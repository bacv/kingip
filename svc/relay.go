package svc

import (
	"io"

	"github.com/quic-go/quic-go"
)

type RelayID uint64

type Client interface {
	io.Reader
	io.Writer
	Close() error
	ID() ClientID
}

type RelayGatewayHandleFunc func(quic.Stream) error
type RelayClientHandleFunc func(Client) error
