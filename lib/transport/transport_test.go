package transport

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	proto "github.com/bacv/kingip/lib/proto"
	"github.com/stretchr/testify/assert"
)

func TestTransportWriteToClosed(t *testing.T) {
	conn, _ := net.Pipe()

	transport := NewTransport(conn, func(w ResponseWriter, r proto.Message) error {
		return nil
	})

	go func() {
		transport.Spawn()
	}()

	transport.Close()
	conn.Close()

	err := transport.Write(proto.Message{})
	assert.Error(t, err, ErrorWriteToClosed, "it should not be possible to write to a clossed transport")
	assert.True(t, transport.IsClosed())
}

func TestTransportHandler(t *testing.T) {
	expected := "1234"
	connA, connB := net.Pipe()

	var sErr error
	serverHandler := func(w ResponseWriter, r proto.Message) error {
		mt, err := r.Type()
		if err != nil {
			sErr = err
			return err
		}

		switch mt {
		case proto.MsgRelayHello:
			_, hello, err := r.UnmarshalMap()
			if err != nil {
				sErr = err
			}

			assert.Equal(t, hello["blue"], "http://blue.com")
			assert.Equal(t, hello["green"], "http://green.com")

			w.Write(proto.NewMsgRelayConfig("1234"))
		}
		return nil
	}

	var cErr error
	var result string
	clientHandler := func(w ResponseWriter, r proto.Message) error {
		mt, err := r.Type()
		if err != nil {
			cErr = err
			return err
		}

		switch mt {
		case proto.MsgRelayConfig:
			_, config, err := r.UnmarshalString()
			if err != nil {
				cErr = err
			}
			result = config
			w.Close()
		}
		return nil
	}

	tA := NewTransport(connA, serverHandler)
	tB := NewTransport(connB, clientHandler)

	done := make(chan struct{})
	go func() {
		defer close(done)
		tA.Spawn()
	}()

	go func() {
		tB.Spawn()
	}()

	go func() {
		err := tB.Write(proto.NewMsgRelayHello(map[string]string{
			"blue":  "http://blue.com",
			"green": "http://green.com",
		}))
		assert.NoError(t, err)
	}()

	var err error
	select {
	case <-time.After(1 * time.Second):
		err = errors.New("timeout")
	case <-done:
		break
	}

	assert.NoError(t, err)
	assert.NoError(t, sErr)
	assert.NoError(t, cErr)
	assert.Equal(t, expected, result, fmt.Sprintf("result should be %s, got %s", expected, result))
}
