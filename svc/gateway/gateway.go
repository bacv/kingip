package gateway

import (
	"errors"
	"io"
	"math/rand"
	"sync"

	"github.com/bacv/kingip/lib/proto"
	quic_kingip "github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type relayConn struct {
	conn  quic.Connection
	stopC chan error
	mu    sync.Mutex
}

func (r *relayConn) openStream() (quic.Stream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.conn.OpenStream()
}

type Gateway struct {
	store      svc.UserStore
	relayConns map[svc.RelayID]*relayConn
	regions    *svc.RegionCache
	mu         sync.RWMutex
}

func NewGateway(store svc.UserStore) *Gateway {
	return &Gateway{
		store:      store,
		relayConns: make(map[svc.RelayID]*relayConn),
		regions:    svc.NewRegionsCache(),
	}
}

func (g *Gateway) RegisterHandle(conn quic.Connection) (svc.RelayID, <-chan error, error) {
	relayId, stopC := g.registerRelay(conn)
	return relayId, stopC, nil
}

func (g *Gateway) RegionsHandle(relayId svc.RelayID, regions map[string]string) error {
	return g.registerRegions(relayId, regions)
}

func (g *Gateway) AuthHandle(name, password string) error {
	user, err := g.store.GetUser(svc.UserAuth{Name: name, Password: password})
	if err != nil {
		return err
	}

	if g.store.GetUserSessionCount(user.ID()) >= user.MaxSessions() {
		return errors.New("User session limit")
	}

	return nil
}

func (g *Gateway) SessionHandle(destination svc.Destination, region svc.Region, userConn svc.Conn) error {
	relayId, ok := g.regions.Get(region)
	if !ok {
		return errors.New("No relay in region")
	}

	stream, err := g.openStream(svc.RelayID(relayId))
	if err != nil {
		return err
	}

	_, err = quic_kingip.SyncTransport(
		stream,
		g.handleProxyInit,
		proto.NewMsgGatewayProxy(string(destination), string(region)),
	)

	go transferData(stream, userConn)
	transferData(userConn, stream)

	return nil
}

func (g *Gateway) handleProxyInit(w quic_kingip.ResponseWriter, r proto.Message) error {
	mt, _, err := r.UnmarshalString()
	if err != nil {
		return err
	}

	if proto.MsgSuccess != mt {
		errors.New("Proxy init failed")
	}

	return nil
}

func (g *Gateway) openStream(relayId svc.RelayID) (quic.Stream, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	relay, ok := g.relayConns[relayId]
	if !ok {
		return nil, errors.New("Relay not found")
	}

	return relay.openStream()
}

func (g *Gateway) registerRelay(conn quic.Connection) (svc.RelayID, chan error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := svc.RelayID(rand.Uint64())

	if _, ok := g.relayConns[id]; !ok {
		stopC := make(chan error)
		g.relayConns[id] = &relayConn{conn: conn, stopC: stopC}
		return id, stopC
	}

	return svc.RelayID(0), nil
}

func (g *Gateway) registerRegions(relayId svc.RelayID, regions map[string]string) error {
	for region, _ := range regions {
		g.regions.Add(svc.Region(region), uint64(relayId))
	}
	return nil
}

func transferData(dst svc.Conn, src svc.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
