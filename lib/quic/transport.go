package quic

import (
	proto "github.com/bacv/kingip/lib/proto"
	"github.com/bacv/kingip/lib/transport"
	"github.com/quic-go/quic-go"
)

func SyncTransport(stream quic.Stream, handler transport.HandleFunc, msg proto.Message) (*transport.Transport, error) {
	transport := transport.NewTransport(stream, handler)
	defer transport.Abandon()

	transport.Write(msg)
	return transport, transport.Sync()
}
