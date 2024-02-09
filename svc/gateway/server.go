package gateway

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/bacv/kingip/lib/proto"
	quic_kingip "github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type ServerConfig struct {
	Addr net.Addr
}

type Server struct {
	config          ServerConfig
	registerHandler svc.GatewayRelayRegisterHandleFunc
	regionsHandler  svc.GatewayRelayRegionsHandleFunc
}

func NewServer(
	ctx context.Context,
	config ServerConfig,
	registerHandler svc.GatewayRelayRegisterHandleFunc,
	regionsHandler svc.GatewayRelayRegionsHandleFunc,
) (*Server, error) {
	return &Server{
		config:          config,
		registerHandler: registerHandler,
		regionsHandler:  regionsHandler,
	}, nil
}

func (s *Server) ListenRelay() error {
	listener, err := quic.ListenAddr(s.config.Addr.String(), quic_kingip.GenerateTLSConfig(), nil)
	if err != nil {
		return err
	}
	log.Println("Server listening on: ", s.config.Addr.String())

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println("Unable to accept conn: ", err)
			continue
		}

		go func() {
			err := s.handleRelayConn(conn)
			if err != nil {
				log.Println("Failed to handle relay conn: ", err)
			}
		}()
	}
}

func (s *Server) handleRelayConn(conn quic.Connection) error {
	for {
		helloStream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		relayId, stopC, err := s.registerHandler(conn)
		if err != nil {
			return err
		}

		// Receive first stream from relay.
		if err := s.handleHelloStream(relayId, helloStream); err != nil && err != io.EOF {
			return err
		}

		// Wait until handler finishes.
		return <-stopC
	}
}

func (s *Server) handleHelloStream(relayId svc.RelayID, stream quic.Stream) error {
	handleHello := func(w quic_kingip.ResponseWriter, r proto.Message) {
		mt, regions, err := r.UnmarshalMap()
		if err != nil {
			log.Println("Failed to parse relay hello: ", err)
			return
		}

		if proto.MsgRelayHello == mt {
			s.regionsHandler(relayId, regions)
			log.Println("Got relay with regions: ", regions)
			w.Write(proto.NewMsgRelayConfig(fmt.Sprint(relayId)))
		} else {
			log.Println("Wrong protocol message")
		}
	}

	transport := quic_kingip.NewTransport(stream, handleHello)
	defer transport.Abandon()

	return transport.Spawn()
}
