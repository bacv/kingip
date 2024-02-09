package relay

import (
	"errors"
	"log"

	"github.com/bacv/kingip/lib/proto"
	quic_kingip "github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type Relay struct {
}

func NewRelay() *Relay {
	return &Relay{}
}

func (r *Relay) ClientHandle(client svc.Client) error {
	return nil
}

func (r *Relay) GatewayHandle(stream quic.Stream) error {
	// Receive proxy destination and region
	_, err := quic_kingip.SyncTransport(
		stream,
		r.handleProxyInit,
		nil,
	)

	if err != nil {
		log.Println("Unable to create proxy", err)
	}

	buffer := make([]byte, 4096)
	n, err := stream.Read(buffer)
	if err != nil {
		log.Println("Error reading from gateway stream:", err)
		return nil
	}
	log.Printf("Relay received from gateway: %s\n", buffer[:n])
	return nil
}

func (r *Relay) handleProxyInit(w quic_kingip.ResponseWriter, rd proto.Message) error {
	mt, proxy, err := rd.UnmarshalMap()
	if err != nil {
		return err
	}

	if proto.MsgGatewayProxy != mt {
		errors.New("Wrong protocol message")
	}

	w.Write(proto.NewMsgSuccess())
	log.Println("Got proxy init request: ", proxy)
	return nil
}
