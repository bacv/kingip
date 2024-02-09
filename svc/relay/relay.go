package relay

import (
	"log"

	"github.com/bacv/kingip/svc"
)

type Relay struct {
}

func NewRelay() *Relay {
	return &Relay{}
}

func (r *Relay) GatewayHandle(stream svc.ReaderWriter) error {
	buffer := make([]byte, 4096)
	n, err := stream.Read(buffer)
	if err != nil {
		log.Println("Error reading from gateway stream:", err)
		return nil
	}
	log.Printf("Relay received from gateway: %s\n", buffer[:n])
	return nil
}

func (r *Relay) ClientHandle(stream svc.ClientReaderWriter) error {
	return nil
}
