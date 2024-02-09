package gateway

import (
	"sync"
	"sync/atomic"

	"github.com/bacv/kingip/svc"
)

type region struct {
	mu     sync.RWMutex
	relays map[svc.RelayID]struct{}
	order  []svc.RelayID
	index  uint64
}

func (r *region) add(relayId svc.RelayID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.relays == nil {
		r.relays = make(map[svc.RelayID]struct{})
	}
	if _, exists := r.relays[relayId]; !exists {
		r.relays[relayId] = struct{}{}
		r.order = append(r.order, relayId)
	}
}

func (r *region) remove(relayId svc.RelayID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.relays[relayId]; exists {
		delete(r.relays, relayId)
		// Remove from order slice
		for i, id := range r.order {
			if id == relayId {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
	}
}

func (r *region) get() (svc.RelayID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.order) == 0 {
		return svc.RelayID(0), false
	}

	index := atomic.AddUint64(&r.index, 1) % uint64(len(r.order))
	return r.order[index], true
}

type regionsCache struct {
	mu      sync.Mutex
	regions map[svc.Region]*region
}

func NewRegionsCache() *regionsCache {
	return &regionsCache{regions: make(map[svc.Region]*region)}
}

func (c *regionsCache) add(regionName svc.Region, relayId svc.RelayID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.regions == nil {
		c.regions = make(map[svc.Region]*region)
	}
	if _, exists := c.regions[regionName]; !exists {
		c.regions[regionName] = &region{}
	}
	c.regions[regionName].add(relayId)
}

func (c *regionsCache) remove(regionName svc.Region, relayId svc.RelayID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if region, exists := c.regions[regionName]; exists {
		region.remove(relayId)
	}
}

func (c *regionsCache) get(regionName svc.Region) (svc.RelayID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if region, exists := c.regions[regionName]; exists {
		return region.get()
	}
	return svc.RelayID(0), false
}
