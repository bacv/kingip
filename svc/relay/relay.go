package relay

import (
	"bufio"
	"errors"
	"io"
	"log"
	"math/rand"
	"sync"

	"github.com/bacv/kingip/lib/proto"
	quic_kingip "github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/lib/transport"
	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type edgeConn struct {
	conn  quic.Connection
	stopC chan error
	mu    sync.Mutex
}

func (e *edgeConn) openStream() (quic.Stream, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.conn.OpenStream()
}

type Relay struct {
	edgeConns map[svc.EdgeID]*edgeConn
	regions   *svc.RegionCache
	mu        sync.RWMutex
}

func NewRelay() *Relay {
	return &Relay{
		edgeConns: make(map[svc.EdgeID]*edgeConn),
		regions:   svc.NewRegionsCache(),
	}
}

func (g *Relay) RegisterHandle(conn quic.Connection) (uint64, <-chan error, error) {
	relayId, stopC := g.registerEdge(conn)
	return uint64(relayId), stopC, nil
}

func (g *Relay) RegionsHandle(id uint64, regions map[string]string) error {
	return g.registerRegions(svc.EdgeID(id), regions)
}

func (r *Relay) GatewayHandle(gatewayStream quic.Stream) error {
	// Receive proxy destination and region
	destination, region, err := getProxyDetails(gatewayStream)
	if err != nil {
		log.Println("Unable to create proxy", err)
	}

	// Pass everything to the edge conn.
	edgeId, ok := r.regions.Get(region)
	if !ok {
		return errors.New("No relay in region")
	}

	edgeStream, err := r.openStream(svc.EdgeID(edgeId))
	if err != nil {
		return err
	}

	_, err = quic_kingip.SyncTransport(
		edgeStream,
		r.handleEdgeStreamInit,
		proto.NewMsgGatewayProxy(string(destination), string(region)),
	)

	go transferData(gatewayStream, edgeStream)
	transferData(edgeStream, gatewayStream)

	return nil

}

func (r *Relay) handleEdgeStreamInit(w transport.ResponseWriter, rd proto.Message) error {
	mt, _, err := rd.UnmarshalMap()
	if err != nil {
		return err
	}

	if proto.MsgSuccess != mt {
		errors.New("Wrong protocol message")
	}

	log.Println("Created edge stream")
	return nil
}

func (g *Relay) openStream(edgeId svc.EdgeID) (quic.Stream, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	relay, ok := g.edgeConns[edgeId]
	if !ok {
		return nil, errors.New("Relay not found")
	}

	return relay.openStream()
}

func (g *Relay) registerEdge(conn quic.Connection) (svc.EdgeID, chan error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := svc.EdgeID(rand.Uint64())

	if _, ok := g.edgeConns[id]; !ok {
		stopC := make(chan error)
		g.edgeConns[id] = &edgeConn{conn: conn, stopC: stopC}
		return id, stopC
	}

	return svc.EdgeID(0), nil
}

func (g *Relay) registerRegions(relayId svc.EdgeID, regions map[string]string) error {
	for region, _ := range regions {
		g.regions.Add(svc.Region(region), uint64(relayId))
	}
	return nil
}

func getProxyDetails(stream quic.Stream) (svc.Destination, svc.Region, error) {
	t := transport.NewTransport(stream, nil)
	defer t.Abandon()

	bytes, err := bufio.NewReader(t).ReadBytes(proto.ByteLF)

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
