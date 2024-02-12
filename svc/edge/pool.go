package edge

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	ErrorMaxHostConns = errors.New("Max limit per host reached")
)

type connDisposable struct {
	conn     net.Conn
	lastUsed time.Time
}

type ConnPool struct {
	pool         map[string][]connDisposable
	connRequests map[string]int
	mu           sync.Mutex
	cond         *sync.Cond
	maxPerHost   int
}

func NewConnPool(maxPerHost int) *ConnPool {
	cp := &ConnPool{
		pool:         make(map[string][]connDisposable),
		connRequests: make(map[string]int),
		maxPerHost:   maxPerHost,
	}
	cp.cond = sync.NewCond(&cp.mu)
	go cp.cleanupIdleConnections(5 * time.Second)
	return cp
}

func (p *ConnPool) Get(destination string) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if conns, ok := p.pool[destination]; ok && len(conns) > 0 {
			connWrap := conns[len(conns)-1]
			p.pool[destination] = conns[:len(conns)-1]
			connWrap.lastUsed = time.Now()
			return connWrap.conn, nil
		}

		if p.canRequestConn(destination) {
			p.connRequests[destination]++
			go p.asyncCreateConn(destination)
			p.cond.Wait()
		} else {
			return nil, ErrorMaxHostConns
		}
	}
}

func (p *ConnPool) Put(destination string, conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pool[destination]) < p.maxPerHost {
		wrap := connDisposable{conn: conn, lastUsed: time.Now()}
		p.pool[destination] = append(p.pool[destination], wrap)
		p.cond.Broadcast()
	} else {
		conn.Close()
	}
}

func (p *ConnPool) asyncCreateConn(destination string) {
	conn, err := net.DialTimeout("tcp", destination, 5*time.Second)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connRequests[destination]--

	if err != nil {
		p.cond.Broadcast()
		return
	}

	if len(p.pool[destination]) < p.maxPerHost {
		p.pool[destination] = append(p.pool[destination], connDisposable{conn: conn, lastUsed: time.Now()})
	} else {
		conn.Close()
	}

	p.cond.Broadcast()
}

func (p *ConnPool) canRequestConn(destination string) bool {
	currentConns := len(p.pool[destination]) + p.connRequests[destination]
	return currentConns < p.maxPerHost
}

func (p *ConnPool) cleanupIdleConnections(idleTimeout time.Duration) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		for destination, conns := range p.pool {
			for i := len(conns) - 1; i >= 0; i-- {
				if time.Since(conns[i].lastUsed) > idleTimeout {
					conns[i].conn.Close()
					conns = append(conns[:i], conns[i+1:]...)
				}
			}
			p.pool[destination] = conns
		}
		p.mu.Unlock()
	}
}
