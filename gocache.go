package gocache

import (
	"sync"
	"time"
)

const (
	// DefaultExpire is default expiration date.
	DefaultExpire time.Duration = 50 * time.Second

	// DeleteExpiredInterval is the default interval at which the worker deltes all expired cache objects
	DeleteExpiredInterval time.Duration = 10 * time.Second

	// DefaultConrurrentMapCount is the number of elements of hashmap.
	DefaultConrurrentMapCount int = 10
)

type concurrentMaps []*concurrentMap

type (

	// Gocache is core gocache interface type.
	Gocache interface {

		// Get returns cache object of given the name.
		Get(string) (interface{}, bool)

		// GetExpire returns expiration date of cache object of given the name.
		GetExpire(string) (int64, bool)

		// Set sets object in th cache.
		Set(string, interface{}) bool

		// SetWithExpire sets object in cache with an expiration date.
		SetWithExpire(string, interface{}, time.Duration) bool

		// Delete deletes cache object of given name.
		Delete(string) bool

		// Clear clears cache.
		Clear()

		// StartDeleteExpired starts worker that deletes an expired cache object.
		// Deletion processing is executed at intervals of given time.
		StartDeleteExpired(dur time.Duration) bool

		// StopDeleteExpired stop worker that deletes an expired cache object.
		StopDeleteExpired() bool
	}

	gocache struct {
		concurrentMaps
	}

	concurrentMap struct {
		m              *sync.Map
		startingWorker bool
		finishWorker   chan bool
	}

	item struct {
		expire int64
		val    interface{}
	}
)

// New returns Gocache (*gocache) instance.
func New() Gocache {
	g := &gocache{
		concurrentMaps: make(concurrentMaps, 0, DefaultConrurrentMapCount),
	}

	for i := 0; i < DefaultConrurrentMapCount; i++ {
		g.concurrentMaps = append(g.concurrentMaps, &concurrentMap{
			m:              new(sync.Map),
			startingWorker: false,
			finishWorker:   make(chan bool),
		})
	}

	return g
}

func (c concurrentMaps) getMap(key string) *concurrentMap {
	return c[fnv32(key)%uint32(DefaultConrurrentMapCount)]
}

func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	for i := 0; i < len(key); i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	return hash
}

func (g *gocache) StartDeleteExpired(dur time.Duration) bool {
	if int(dur) <= 0 {
		return false
	}

	g.StopDeleteExpired()

	for _, c := range g.concurrentMaps {
		go c.start(dur)
	}

	return true
}

func (g *gocache) StopDeleteExpired() bool {
	for _, c := range g.concurrentMaps {
		c.finishWorker <- true
		c.startingWorker = true
	}

	return true
}

func (c *concurrentMap) start(dur time.Duration) {
	go func() {
		t := time.NewTicker(dur)

	END_LOOP:
		for {
			select {
			case _ = <-c.finishWorker:
				break END_LOOP
			case _ = <-t.C:
				c.deleteExpired()
			}
		}
	}()
}

func (g *item) isValid() bool {
	return time.Now().UnixNano() < g.expire
}

func (g *gocache) Get(key string) (interface{}, bool) {
	c := g.concurrentMaps.getMap(key)

	item, ok := c.get(key)
	if !ok {
		return nil, false
	}

	return item.val, ok
}

func (g *gocache) GetExpire(key string) (int64, bool) {
	c := g.concurrentMaps.getMap(key)

	item, ok := c.get(key)
	if !ok {
		return 0, false
	}

	return item.expire, ok
}

func (c *concurrentMap) get(key string) (item, bool) {
	value, ok := c.m.Load(key)
	if ok {
		i := value.(item)
		if i.isValid() {
			return i, ok
		}

		c.m.Delete(key)
	}

	return item{}, false
}

func (g *gocache) Set(key string, val interface{}) bool {
	c := g.concurrentMaps.getMap(key)
	return c.set(key, val, DefaultExpire)
}

func (g *gocache) SetWithExpire(key string, val interface{}, expire time.Duration) bool {
	c := g.concurrentMaps.getMap(key)
	return c.set(key, val, expire)
}

func (c *concurrentMap) set(key string, val interface{}, expire time.Duration) bool {
	if expire <= 0 {
		return false
	}

	c.m.Store(key, item{
		val:    val,
		expire: time.Now().Add(expire).UnixNano(),
	})

	return true
}

func (g *gocache) Delete(key string) bool {
	c := g.concurrentMaps.getMap(key)
	return c.delete(key)
}

func (g *gocache) DeleteExpired() {
	for _, concurrentMap := range g.concurrentMaps {
		concurrentMap.deleteExpired()
	}
}

func (c *concurrentMap) deleteExpired() {
	c.m.Range(func(key interface{}, val interface{}) bool {
		item := val.(item)

		if !item.isValid() {
			c.m.Delete(key)
		}
		return true
	})
}

func (c *concurrentMap) delete(key string) bool {
	_, ok := c.m.Load(key)
	if !ok {
		return false
	}

	c.m.Delete(key)

	return true
}

func (c *concurrentMap) deleteAll() {
	c.m.Range(func(key interface{}, val interface{}) bool {
		c.m.Delete(key)
		return true
	})
}

func (g *gocache) Clear() {
	for i := 0; i < len(g.concurrentMaps); i++ {
		c := g.concurrentMaps[i]
		if c.startingWorker {
			c.finishWorker <- true
			c.startingWorker = false
		}

		c.deleteAll()
	}
}
