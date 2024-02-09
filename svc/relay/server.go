package relay

import (
	"context"
	"errors"
	"log"
	"net"

	"github.com/bacv/kingip/lib/proto"
	quic_kingip "github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type Client struct {
	conn svc.Conn
}

type Config struct {
	DialGatewayAddr  net.Addr
	ListenClientAddr net.Addr
	Regions          map[string]string
}

// Connects to Gateway, accepts connections from Clients.
type Server struct {
	config         Config
	clients        map[svc.ClientID]quic.Stream
	gateways       map[string]svc.RelayID
	clientHandler  svc.RelayClientHandleFunc
	gatewayHandler svc.RelayGatewayHandleFunc
}

func NewServer(
	config Config,
	clientHandler svc.RelayClientHandleFunc,
	gatewayHandler svc.RelayGatewayHandleFunc,
) *Server {
	return &Server{
		config:         config,
		clients:        make(map[svc.ClientID]quic.Stream),
		clientHandler:  clientHandler,
		gatewayHandler: gatewayHandler,
	}
}

func (s *Server) DialGateway(ctx context.Context) error {
	conn, err := quic.DialAddrEarly(
		ctx, s.config.DialGatewayAddr.String(),
		quic_kingip.TlsClientConfig.Clone(),
		nil,
	)
	if err != nil {
		return err
	}

	configStream, err := conn.OpenStream()
	if err != nil {
		return err
	}

	// Register relay on gateway to receive configuration.
	transport, err := quic_kingip.SyncTransport(
		configStream,
		s.handleConfig,
		proto.NewMsgRelayHello(s.config.Regions),
	)
	transport.Close()

	if err != nil {
		return err
	}

	// Listen for new streams comming from gateway.
	return s.listenGatewayStreams(conn)
}

func (s *Server) handleConfig(w quic_kingip.ResponseWriter, r proto.Message) error {
	mt, id, err := r.UnmarshalString()
	if err != nil {
		return err
	}

	if proto.MsgRelayConfig != mt {
		errors.New("Wrong protocol message")
	}

	log.Println("Got id from gateway: ", id)
	return nil
}

func (s *Server) listenGatewayStreams(conn quic.Connection) error {
	for {
		proxyStream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		go func() {
			if err := s.gatewayHandler(proxyStream); err != nil {
				log.Print(err)
			}
		}()
	}
}
