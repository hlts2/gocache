package gocache

import (
	"sync"
	"time"
	"unsafe"

	"github.com/cespare/xxhash"
)

const (
	// DefaultExpire is default expiration date.
	DefaultExpire time.Duration = 50 * time.Second

	// DeleteExpiredInterval is the default interval at which the worker deltes all expired cache objects
	DeleteExpiredInterval time.Duration = 10 * time.Second

	// DefaultConcurrentMapCount is the number of elements of concurrent map.
	DefaultConcurrentMapCount uint64 = 10
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
		StartDeleteExpired(dur time.Duration) Gocache

		// StopDeleteExpired stop worker that deletes an expired cache object.
		StopDeleteExpired() Gocache

		// StartingDeleteExpired returns true if the worker that deletes expired item is running, returns false otherwise.
		StartingDeleteExpired() bool
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
		concurrentMaps: make(concurrentMaps, 0, DefaultConcurrentMapCount),
	}

	for i := 0; i < int(DefaultConcurrentMapCount); i++ {
		g.concurrentMaps = append(g.concurrentMaps, &concurrentMap{
			m:              new(sync.Map),
			startingWorker: false,
			finishWorker:   make(chan bool),
		})
	}

	return g
}

func (g *gocache) Get(key string) (interface{}, bool) {
	cm := g.concurrentMaps.getMap(key)

	item, ok := cm.get(key)
	if !ok {
		return nil, false
	}

	return item.val, ok
}

func (g *gocache) GetExpire(key string) (int64, bool) {
	cm := g.concurrentMaps.getMap(key)

	item, ok := cm.get(key)
	if !ok {
		return 0, false
	}

	return item.expire, ok
}

func (g *gocache) Set(key string, val interface{}) bool {
	cm := g.concurrentMaps.getMap(key)
	return cm.set(key, val, DefaultExpire)
}

func (g *gocache) SetWithExpire(key string, val interface{}, expire time.Duration) bool {
	cm := g.concurrentMaps.getMap(key)
	return cm.set(key, val, expire)
}

func (g *gocache) Delete(key string) bool {
	cm := g.concurrentMaps.getMap(key)
	return cm.delete(key)
}

func (g *gocache) DeleteExpired() {
	for _, cm := range g.concurrentMaps {
		cm.deleteExpired()
	}
}

func (g *gocache) Clear() {
	for _, cm := range g.concurrentMaps {
		cm.deleteAll()
	}
}

func (g *gocache) StartDeleteExpired(dur time.Duration) Gocache {
	if int(dur) <= 0 {
		return g
	}

	for _, cm := range g.concurrentMaps {
		if cm.startingWorker {
			return g
		}
	}

	for _, cm := range g.concurrentMaps {
		cm.startingWorker = true
		go cm.start(dur)
	}

	return g
}

func (g *gocache) StopDeleteExpired() Gocache {
	for _, cm := range g.concurrentMaps {
		if cm.startingWorker {
			cm.finishWorker <- true
			cm.startingWorker = false
		}
	}

	return g
}

func (g *gocache) StartingDeleteExpired() bool {
	for _, cm := range g.concurrentMaps {
		if cm.startingWorker {
			return true
		}
	}
	return false
}

func (g *item) isValid() bool {
	return time.Now().UnixNano() < g.expire
}

func (cms concurrentMaps) getMap(key string) *concurrentMap {
	return cms[xxhash.Sum64(*(*[]byte)(unsafe.Pointer(&key)))%DefaultConcurrentMapCount]
}

func (cm *concurrentMap) get(key string) (item, bool) {
	value, ok := cm.m.Load(key)
	if ok {
		i := value.(item)
		if i.isValid() {
			return i, ok
		}

		cm.m.Delete(key)
	}

	return item{}, false
}

func (cm *concurrentMap) set(key string, val interface{}, expire time.Duration) bool {
	if expire <= 0 {
		return false
	}

	cm.m.Store(key, item{
		val:    val,
		expire: time.Now().Add(expire).UnixNano(),
	})

	return true
}

func (cm *concurrentMap) delete(key string) bool {
	_, ok := cm.m.Load(key)
	if !ok {
		return false
	}

	cm.m.Delete(key)

	return true
}

func (cm *concurrentMap) deleteAll() {
	cm.m.Range(func(key interface{}, val interface{}) bool {
		cm.m.Delete(key)
		return true
	})
}

func (cm *concurrentMap) deleteExpired() {
	cm.m.Range(func(key interface{}, val interface{}) bool {
		item := val.(item)

		if !item.isValid() {
			cm.m.Delete(key)
		}
		return true
	})
}

func (cm *concurrentMap) start(dur time.Duration) {
	go func() {
		t := time.NewTicker(dur)

	END_LOOP:
		for {
			select {
			case _ = <-cm.finishWorker:
				break END_LOOP
			case _ = <-t.C:
				cm.deleteExpired()
			}
		}
	}()
}
