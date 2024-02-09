package quic

import (
	"context"
	"errors"
	"log"
	"net"

	proto "github.com/bacv/kingip/lib/proto"
	"github.com/bacv/kingip/lib/transport"
	"github.com/quic-go/quic-go"
)

type DialerStreamHandleFunc func(quic.Stream) error

type DialerConfig struct {
	Addr    net.Addr
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
	conn, err := quic.DialAddrEarly(
		ctx, s.config.Addr.String(),
		TlsClientConfig.Clone(),
		nil,
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

	// Listen for new streams comming from the server.
	return s.listenStreams(conn)
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

func (s *Dialer) listenStreams(conn quic.Connection) error {
	for {
		stream, err := conn.AcceptStream(context.Background())
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
