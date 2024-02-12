package gateway

import (
	"errors"
	"io"
	"log"
	"math/rand"
	"sync"
	"time"

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
	bandwidthStore svc.BandwidthStore
	userStore      svc.UserStore
	sessionStore   svc.SessionStore

	relayConns map[svc.RelayID]*relayConn
	regions    *svc.RegionCache
	mu         sync.RWMutex
}

func NewGateway(userStore svc.UserStore, bandwidthStore svc.BandwidthStore, sessionStore svc.SessionStore) *Gateway {
	return &Gateway{
		userStore:      userStore,
		bandwidthStore: bandwidthStore,
		sessionStore:   sessionStore,

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

func (g *Gateway) AuthHandle(name, password string) (*svc.User, error) {
	user, err := g.userStore.GetUser(svc.UserAuth{Name: name, Password: password})
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (g *Gateway) SessionHandle(user *svc.User, destination svc.Destination, region svc.Region, userConn svc.Conn) error {
	log.Print("Connecting to: ", destination)

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

	sessions := g.sessionStore.SessionAdd(user.ID())
	defer g.sessionStore.SessionRemove(user.ID())

	if sessions > user.MaxSessions() {
		return errors.New("Max sessions")
	}

	mbs := g.bandwidthStore.GetUserTotalUsedMBs(user.ID())
	if mbs/1024. > user.MaxGBs() {
		return errors.New("Max bandwidth used")
	}

	inboundC := make(chan transferResult)
	go func() {
		inbound, err := transferData(userConn, relayStream)
		inboundC <- transferResult{bytesCopied: inbound, err: err}
	}()

	outboundC := make(chan transferResult)
	go func() {
		outbound, err := transferData(relayStream, userConn)
		outboundC <- transferResult{bytesCopied: outbound, err: err}
	}()

	select {
	case inboundRes := <-inboundC:
		g.receiveIoRes(user, inboundRes, outboundC)
		return nil
	case outboundRes := <-outboundC:
		g.receiveIoRes(user, outboundRes, inboundC)
		return nil
	case <-time.After(user.MaxSessionDuration()):
		userConn.Close()
		relayStream.Close()
		return errors.New("Max session duration")
	}
}

func (g *Gateway) receiveIoRes(user *svc.User, res transferResult, otherC <-chan transferResult) (float64, error) {
	origin, err := res.bytesCopied, res.err
	if err != nil {
		return 0, err
	}
	otherRes := <-otherC
	other, err := otherRes.bytesCopied, otherRes.err

	totalBytes := origin + other
	megabytes := float64(totalBytes) / (1024.0 * 1024.0)

	g.bandwidthStore.UpdateUserTotalUsedMBs(user.ID(), megabytes)
	return 0, err
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
