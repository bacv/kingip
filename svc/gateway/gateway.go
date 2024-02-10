package gateway

import (
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

type relayConn struct {
	conn    quic.Connection
	stopC   chan error
	regions []svc.Region
	mu      sync.Mutex
}

func (r *relayConn) updateRegions(regions []svc.Region) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.regions = regions
}

func (r *relayConn) getRegions() []svc.Region {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.regions
}

func (r *relayConn) openStream() (quic.Stream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.conn.OpenStream()
}

func (r *relayConn) stop() {
	close(r.stopC)
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

func (g *Gateway) RegisterHandle(conn quic.Connection) (uint64, <-chan error, error) {
	relayId, stopC := g.registerRelay(conn)
	return uint64(relayId), stopC, nil
}

func (g *Gateway) RegionsHandle(id uint64, regions map[string]string) error {
	return g.registerRegions(svc.RelayID(id), regions)
}

func (g *Gateway) CloseHandle(id uint64) {
	regions := g.closeRelay(svc.RelayID(id))
	for _, region := range regions {
		g.regions.Remove(region, id)
	}
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

	relayStream, err := g.openStream(svc.RelayID(relayId))
	if err != nil {
		g.stopRelay(svc.RelayID(relayId))
		return err
	}

	if _, err = quic_kingip.SyncTransport(
		relayStream,
		g.handleProxyInit,
		proto.NewMsgGatewayProxy(string(destination), string(region)),
	); err != nil {
		return err
	}

	inboundC := make(chan transferResult)
	go func() {
		inbound, err := transferData(userConn, relayStream)
		inboundC <- transferResult{bytesCopied: inbound, err: err}
	}()

	outbound, err := transferData(relayStream, userConn)
	if err != nil {
		return err
	}

	inboundRes := <-inboundC
	inbound, err := inboundRes.bytesCopied, inboundRes.err
	if err != nil {
		return err
	}

	log.Printf(">>> inbound: %d; outbound: %d >>>", inbound, outbound)

	return nil
}

func (g *Gateway) handleProxyInit(w transport.ResponseWriter, r proto.Message) error {
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

func (g *Gateway) stopRelay(id svc.RelayID) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if relay, ok := g.relayConns[id]; ok {
		relay.stop()
	}
}

func (g *Gateway) closeRelay(id svc.RelayID) []svc.Region {
	g.mu.Lock()
	defer g.mu.Unlock()

	if relay, ok := g.relayConns[id]; ok {
		regions := relay.getRegions()
		delete(g.relayConns, id)
		return regions
	}

	return nil
}

func (g *Gateway) registerRegions(relayId svc.RelayID, regions map[string]string) error {
	var relayRegions []svc.Region
	for region, _ := range regions {
		g.regions.Add(svc.Region(region), uint64(relayId))
		relayRegions = append(relayRegions, svc.Region(region))
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.relayConns[relayId].updateRegions(relayRegions)
	return nil
}

type transferResult struct {
	bytesCopied int64
	err         error
}

func transferData(dst svc.Conn, src svc.Conn) (int64, error) {
	defer dst.Close()
	defer src.Close()

	bytesCopied, err := io.Copy(dst, src)
	if err != nil {
		return bytesCopied, err
	}

	return bytesCopied, nil
}
