package relay

import (
	"context"
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
	conn, err := quic.DialAddrEarly(ctx, s.config.DialGatewayAddr.String(), quic_kingip.TlsClientConfig.Clone(), nil)
	if err != nil {
		return err
	}

	configStream, err := conn.OpenStream()
	if err != nil {
		return err
	}

	// Register relay on gateway to receive configuration.
	if err := s.handleConfigStream(configStream); err != nil {
		return err
	}

	// Listen for new streams comming from gateway.
	return s.handleGatewayProxyStreams(conn)
}

func (s *Server) handleConfigStream(stream quic.Stream) error {
	transport := quic_kingip.NewTransport(stream, s.handleConfig)
	defer transport.Close()

	transport.Write(proto.NewMsgRelayHello(s.config.Regions))
	return transport.Spawn()
}

func (s *Server) handleConfig(w quic_kingip.ResponseWriter, r proto.Message) {
	defer w.Close()

	mt, id, err := r.UnmarshalString()
	if err != nil {
		log.Println("Failed to parse configuration: ", err)
		return
	}

	if proto.MsgRelayConfig == mt {
		log.Println("Got id from gateway: ", id)
		//s.gateways[s.config.DialGatewayAddr.String()] = svc.RelayID(id)
	} else {
		log.Println("Wrong protocol message")
	}
}

func (s *Server) handleGatewayProxyStreams(conn quic.Connection) error {
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		go func() {
			err := s.gatewayHandler(stream)
			if err != nil {
				log.Print(err)
			}
		}()
	}
}
