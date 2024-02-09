package svc

import "io"

type RelayID uint64

type ReaderWriter interface {
	io.Writer
	io.Reader
}

type ClientReaderWriter interface {
	ReaderWriter
	ID() ClientID
}

type RelayGatewayHandleFunc func(ReaderWriter) error
type RelayClientHandleFunc func(ClientReaderWriter) error
