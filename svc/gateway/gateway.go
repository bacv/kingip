package gateway

import (
	"errors"
	"io"
	"math/rand"

	"github.com/bacv/kingip/svc"
	"github.com/quic-go/quic-go"
)

type relayConn struct {
	conn  quic.Connection
	stopC chan error
}

type Gateway struct {
	store      svc.UserStore
	relayConns map[svc.RelayID]relayConn
	regions    *regionsCache
}

func NewGateway(store svc.UserStore) *Gateway {
	return &Gateway{
		store:      store,
		relayConns: make(map[svc.RelayID]relayConn),
		regions:    NewRegionsCache(),
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

func (g *Gateway) SessionHandle(region svc.Region, userConn svc.Conn) error {
	// here i need to create new stream and assign one end to the proxy, another to the session?
	return nil
}

func (g *Gateway) registerRelay(conn quic.Connection) (svc.RelayID, chan error) {
	for {
		id := svc.RelayID(rand.Uint64())

		if _, ok := g.relayConns[id]; !ok {
			stopC := make(chan error)
			g.relayConns[id] = relayConn{conn: conn, stopC: stopC}
			return id, stopC
		}
	}
}

func (g *Gateway) registerRegions(relayId svc.RelayID, regions map[string]string) error {
	for region, _ := range regions {
		g.regions.add(svc.Region(region), relayId)
	}
	return nil
}

func transferData(dst svc.Conn, src svc.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
