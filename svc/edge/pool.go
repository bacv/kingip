package edge

import (
	"errors"
	"net"
	"sync"
)

var (
	ErrorMaxHostConns = errors.New("Max limit per host reached")
)

type ConnPool struct {
	pool         map[string][]net.Conn
	connRequests map[string]int
	mu           sync.Mutex
	cond         *sync.Cond
	maxPerHost   int
}

func NewConnPool(maxPerHost int) *ConnPool {
	cp := &ConnPool{
		pool:         make(map[string][]net.Conn),
		connRequests: make(map[string]int),
		maxPerHost:   maxPerHost,
	}
	cp.cond = sync.NewCond(&cp.mu)
	return cp
}

func (p *ConnPool) Get(destination string) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if conns, ok := p.pool[destination]; ok && len(conns) > 0 {
			conn := conns[len(conns)-1]
			p.pool[destination] = conns[:len(conns)-1]
			return conn, nil
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
		p.pool[destination] = append(p.pool[destination], conn)
		p.cond.Broadcast()
	} else {
		conn.Close()
	}
}

func (p *ConnPool) asyncCreateConn(destination string) {
	conn, err := net.Dial("tcp", destination)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connRequests[destination]--

	if err != nil {
		p.cond.Broadcast()
		return
	}

	if len(p.pool[destination]) < p.maxPerHost {
		p.pool[destination] = append(p.pool[destination], conn)
	} else {
		conn.Close()
	}

	p.cond.Broadcast()
}

func (p *ConnPool) canRequestConn(destination string) bool {
	currentConns := len(p.pool[destination]) + p.connRequests[destination]
	return currentConns < p.maxPerHost
}
