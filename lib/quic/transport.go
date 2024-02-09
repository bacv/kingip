package quic

import (
	"bufio"
	"errors"
	"io"
	"sync"

	"github.com/bacv/kingip/lib/proto"
)

var (
	ErrorSendChannelClosed = errors.New("Send channel is closed")
	ErrorWriteToClosed     = errors.New("Writing to a closed transport")
)

type Conn interface {
	io.Reader
	io.Writer
	Close() error
}

type ResponseWriter interface {
	Write(proto.Message) error
	Close()
}

type HandleFunc func(ResponseWriter, proto.Message)

type Transport struct {
	conn      Conn
	stopC     chan struct{}
	handler   HandleFunc
	closeOnce sync.Once
	closed    bool
	mu        sync.RWMutex
}

func NewTransport(conn Conn, handler HandleFunc) *Transport {
	return &Transport{
		conn:    conn,
		handler: handler,
		stopC:   make(chan struct{}),
	}
}

func (t *Transport) Spawn() error {
	defer t.Close()
	errC := make(chan error)

	go func() {
		t.read(errC)
	}()

	err := <-errC
	return err
}

// Closes transport **AND** underlying connection.
func (t *Transport) Close() {
	t.closeOnce.Do(func() {
		t.close()
		t.conn.Close()
	})
}

// Abandons connection and closes the transport.
func (t *Transport) Abandon() {
	t.closeOnce.Do(func() {
		t.close()
	})
}

func (t *Transport) close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	close(t.stopC)
}

func (t *Transport) Write(msg proto.Message) error {
	if t.IsClosed() {
		return ErrorWriteToClosed
	}

	t.conn.Write(msg)
	return nil
}

func (t *Transport) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.closed
}

func (t *Transport) read(errC chan<- error) {
	for {
		select {
		case <-t.stopC:
			return
		default:
			bytes, err := bufio.NewReader(t.conn).ReadBytes(proto.ByteLF)

			if err != nil {
				errC <- err
				return
			}

			t.handler(t, proto.Message(bytes))
		}
	}
}
