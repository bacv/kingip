package quic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	proto "github.com/bacv/kingip/lib/proto"
	"github.com/bacv/kingip/lib/transport"
	"github.com/quic-go/quic-go"
)

type ListenerRegisterHandleFunc func(quic.Connection) (uint64, <-chan error, error)
type ListenerRegionsHandleFunc func(uint64, map[string]string) error
type ListenerCloseHandleFunc func(uint64)

type ListenerConfig struct {
	Addr string
}

type Listener struct {
	config          ListenerConfig
	registerHandler ListenerRegisterHandleFunc
	regionsHandler  ListenerRegionsHandleFunc
	closeHandler    ListenerCloseHandleFunc
}

func NewListener(
	ctx context.Context,
	config ListenerConfig,
	registerHandler ListenerRegisterHandleFunc,
	regionsHandler ListenerRegionsHandleFunc,
	closeHandler ListenerCloseHandleFunc,
) *Listener {
	return &Listener{
		config:          config,
		registerHandler: registerHandler,
		regionsHandler:  regionsHandler,
		closeHandler:    closeHandler,
	}
}

func (s *Listener) Listen() error {
	listener, err := quic.ListenAddr(s.config.Addr, GenerateTLSConfig(), &quic.Config{
		MaxIncomingStreams: 100_000,
	})
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println("Unable to accept conn: ", err)
			continue
		}

		go s.acceptConn(conn)
	}
}

func (s *Listener) acceptConn(conn quic.Connection) {
	pingStream, err := conn.OpenStream()
	if err != nil {
		log.Println("Failed to open ping stream: ", err)
		return
	}

	id, stopC, err := s.handleConn(conn)
	if err != nil {
		log.Println("Failed to handle conn: ", err)
		return
	}

	pingC, err := s.ping(id, pingStream)
	if err != nil {
		log.Println("Failed to spawn ping", err)
		return
	}

	select {
	case <-pingC:
		log.Println("Ping timeout")
	case err := <-stopC:
		if err != nil {
			log.Println("Relay closed with err: ", err)
		}
	}

	s.closeHandler(id)
}

func (s *Listener) handleConn(conn quic.Connection) (uint64, <-chan error, error) {
	for {
		helloStream, err := conn.AcceptStream(context.Background())
		defer helloStream.Close()

		if err != nil {
			return 0, nil, err
		}

		id, stopC, err := s.registerHandler(conn)
		if err != nil {
			return 0, nil, err
		}

		// Receive first stream from dialer.
		if err := s.handleHelloStream(id, helloStream); err != nil && err != io.EOF {
			return 0, nil, err
		}

		// Wait until handler finishes.
		return id, stopC, nil
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

	_, err := SyncTransport(
		stream,
		handleHello,
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

func (s *Listener) ping(id uint64, pingStream quic.Stream) (<-chan struct{}, error) {
	stopC := make(chan struct{})
	pingStream.SetReadDeadline(time.Now().Add(time.Second))

	// First ping needs to be sent right away to "claim" this stream.
	if _, err := SyncTransport(pingStream, pingHandler, proto.NewMsgPing(fmt.Sprint(id))); err != nil {
		return nil, err
	}

	go func() {
		defer close(stopC)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C

			pingStream.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, err := SyncTransport(pingStream, pongHandler, proto.NewMsgPing(fmt.Sprint(id))); err != nil {
				return
			}
		}
	}()

	return stopC, nil
}

func pongHandler(w transport.ResponseWriter, r proto.Message) error {
	mt, _, err := r.UnmarshalString()
	if err != nil {
		return err
	}

	if proto.MsgPing != mt {
		return errors.New("Wrong protocol message")
	}

	return nil
}
