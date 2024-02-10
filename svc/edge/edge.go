package edge

import (
	"bufio"
	"errors"
	"io"
	"log"
	"time"

	"github.com/bacv/kingip/lib/proto"
	"github.com/bacv/kingip/lib/transport"
	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type Edge struct {
	connPool *ConnPool
}

func NewEdge() *Edge {
	return &Edge{connPool: NewConnPool(999)}
}

func (r *Edge) RelayHandle(relayStream quic.Stream) error {
	// Receive proxy destination and region.
	destination, _, err := getProxyDetails(relayStream)
	if err != nil {
		log.Println("Unable to create proxy", err)
	}

	if err != nil {
		log.Println("Unable to create proxy", err)
	}

	//destConn, err := net.Dial("tcp", string(destination))
	destConn, err := r.connPool.Get(string(destination))
	if err != nil {
		if err != ErrorMaxHostConns {
			log.Printf("Error connecting to destination [%s]: %v", destination, err)
			return err
		}

		<-time.After(50 * time.Millisecond)
		destConn, err = r.connPool.Get(string(destination))

		if err != nil {
			log.Printf("Error connecting to destination [%s]: %v", destination, err)
			return err
		}
	}

	log.Printf("Created connection to [%s]", destination)

	go transferData(relayStream, destConn)
	transferData(destConn, relayStream)

	return nil
}

func getProxyDetails(stream quic.Stream) (svc.Destination, svc.Region, error) {
	t := transport.NewTransport(stream, nil)
	defer t.Abandon()
	bufReader := bufio.NewReader(t)
	bytes, err := bufReader.ReadSlice(proto.ByteLF)

	mt, proxy, err := proto.Message(bytes).UnmarshalMap()
	if err != nil {
		return "", "", err
	}

	if proto.MsgGatewayProxy != mt {
		return "", "", errors.New("Wrong protocol message")
	}

	t.Write(proto.NewMsgSuccess())
	return svc.Destination(proxy["destination"]), svc.Region(proxy["region"]), nil
}

func transferData(dst svc.Conn, src svc.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
