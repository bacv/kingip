package quic

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"

	proto "github.com/bacv/kingip/lib/proto"
	"github.com/bacv/kingip/lib/transport"
	"github.com/quic-go/quic-go"
)

type ListenerRegisterHandleFunc func(quic.Connection) (uint64, <-chan error, error)
type ListenerRegionsHandleFunc func(uint64, map[string]string) error

type ListenerConfig struct {
	Addr net.Addr
}

type Listener struct {
	config          ListenerConfig
	registerHandler ListenerRegisterHandleFunc
	regionsHandler  ListenerRegionsHandleFunc
}

func NewListener(
	ctx context.Context,
	config ListenerConfig,
	registerHandler ListenerRegisterHandleFunc,
	regionsHandler ListenerRegionsHandleFunc,
) *Listener {
	return &Listener{
		config:          config,
		registerHandler: registerHandler,
		regionsHandler:  regionsHandler,
	}
}

func (s *Listener) Listen() error {
	listener, err := quic.ListenAddr(s.config.Addr.String(), GenerateTLSConfig(), nil)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println("Unable to accept conn: ", err)
			continue
		}

		go func() {
			err := s.handleConn(conn)
			if err != nil {
				log.Println("Failed to handle conn: ", err)
			}
		}()
	}
}

func (s *Listener) handleConn(conn quic.Connection) error {
	for {
		helloStream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		id, stopC, err := s.registerHandler(conn)
		if err != nil {
			return err
		}

		// Receive first stream from dialer.
		if err := s.handleHelloStream(id, helloStream); err != nil && err != io.EOF {
			return err
		}

		// Wait until handler finishes.
		return <-stopC
	}
}

func (s *Listener) handleHelloStream(id uint64, stream quic.Stream) error {
	handleHello := func(w transport.ResponseWriter, r proto.Message) error {
		mt, regions, err := r.UnmarshalMap()
		if err != nil {
			return err
		}

		if proto.MsgRelayHello != mt {
			log.Println("Wrong protocol message")
		}

		s.regionsHandler(id, regions)
		log.Println("Got id with regions: ", regions)
		w.Write(proto.NewMsgRelayConfig(fmt.Sprint(id)))
		return nil
	}

	transport := transport.NewTransport(stream, handleHello)
	defer transport.Abandon()

	return transport.Spawn()
}
