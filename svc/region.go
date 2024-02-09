package svc

import (
	"sync"
	"sync/atomic"
)

type region struct {
	mu    sync.RWMutex
	conns map[uint64]struct{}
	order []uint64
	index uint64
}

func (r *region) add(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conns == nil {
		r.conns = make(map[uint64]struct{})
	}
	if _, exists := r.conns[id]; !exists {
		r.conns[id] = struct{}{}
		r.order = append(r.order, id)
	}
}

func (r *region) remove(connId uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.conns[connId]; exists {
		delete(r.conns, connId)

		// Update order slice.
		for i, id := range r.order {
			if id == connId {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
	}
}

func (r *region) get() (uint64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.order) == 0 {
		return 0, false
	}

	index := atomic.AddUint64(&r.index, 1) % uint64(len(r.order))
	return r.order[index], true
}

type RegionCache struct {
	mu      sync.Mutex
	regions map[Region]*region
}

func NewRegionsCache() *RegionCache {
	return &RegionCache{regions: make(map[Region]*region)}
}

func (c *RegionCache) Add(regionName Region, connId uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.regions == nil {
		c.regions = make(map[Region]*region)
	}
	if _, exists := c.regions[regionName]; !exists {
		c.regions[regionName] = &region{}
	}
	c.regions[regionName].add(connId)
}

func (c *RegionCache) Remove(regionName Region, connId uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if region, exists := c.regions[regionName]; exists {
		region.remove(connId)
	}
}

func (c *RegionCache) Get(regionName Region) (uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if region, exists := c.regions[regionName]; exists {
		return region.get()
	}
	return 0, false
}
