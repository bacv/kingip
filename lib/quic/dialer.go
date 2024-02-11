package quic

import (
	"context"
	"errors"
	"log"
	"time"

	proto "github.com/bacv/kingip/lib/proto"
	"github.com/bacv/kingip/lib/transport"
	"github.com/quic-go/quic-go"
)

type DialerStreamHandleFunc func(quic.Stream) error

type DialerConfig struct {
	Addr    string
	Regions map[string]string
}

type Dialer struct {
	config        DialerConfig
	streamHandler DialerStreamHandleFunc
}

func NewDialer(
	config DialerConfig,
	streamHandler DialerStreamHandleFunc,
) *Dialer {
	return &Dialer{
		config:        config,
		streamHandler: streamHandler,
	}
}

func (s *Dialer) Dial(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	conn, err := quic.DialAddrEarly(
		ctx, s.config.Addr,
		TlsClientConfig.Clone(),
		&quic.Config{
			MaxIncomingStreams: 100_000,
		},
	)
	if err != nil {
		return err
	}

	configStream, err := conn.OpenStream()
	if err != nil {
		return err
	}

	transport, err := SyncTransport(
		configStream,
		s.handleConfig,
		proto.NewMsgRelayHello(s.config.Regions),
	)
	transport.Close()

	if err != nil {
		return err
	}

	pingStream, err := conn.AcceptStream(context.Background())
	go s.pong(pingStream, cancel)

	// Listen for new streams comming from the server.
	return s.listenStreams(ctx, conn)
}

func (s *Dialer) handleConfig(w transport.ResponseWriter, r proto.Message) error {
	mt, id, err := r.UnmarshalString()
	if err != nil {
		return err
	}

	if proto.MsgRelayConfig != mt {
		errors.New("Wrong protocol message")
	}

	log.Println("Got id from stream: ", id)
	return nil
}

func (s *Dialer) listenStreams(ctx context.Context, conn quic.Connection) error {
	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			return err
		}

		go func() {
			if err := s.streamHandler(stream); err != nil {
				log.Print(err)
			}
		}()
	}
}

func (s *Dialer) pong(pingStream quic.Stream, cancel context.CancelFunc) error {
	pingStream.SetReadDeadline(time.Now().Add(5 * time.Second))

	if _, err := SyncTransport(pingStream, pingHandler, nil); err != nil {
		return err
	}

	go func() {
		defer cancel()
		for {
			pingStream.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, err := SyncTransport(pingStream, pingHandler, nil); err != nil {
				return
			}
		}
	}()

	return nil
}

func pingHandler(w transport.ResponseWriter, r proto.Message) error {
	mt, id, err := r.UnmarshalString()
	if err != nil {
		return err
	}

	if proto.MsgPing != mt {
		return errors.New("Wrong protocol message")
	}

	w.Write(proto.NewMsgPing(id))
	return nil
}
