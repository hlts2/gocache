package gocache

import (
	"sync"
	"time"
	"unsafe"

	"github.com/cespare/xxhash"
	"github.com/kpango/fastime"
)

const (
	// DefaultExpire is default expiration date.
	DefaultExpire time.Duration = 50 * time.Second

	// DeleteExpiredInterval is the default interval at which the worker deltes all expired cache objects
	DeleteExpiredInterval time.Duration = 10 * time.Second

	// DefaultConcurrentMapCount is the number of elements of concurrent map.
	DefaultConcurrentMapCount int = 256
)

// Gocache is base gocache interface.
type Gocache interface {

	// Get returns object with the given name from the cache.
	Get(string) (interface{}, bool)

	// GetExpire returns expiration date of cache object of given the name.
	GetExpire(string) (int64, bool)

	// Set sets object in the cache.
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

	// StartingDeleteExpired returns true if the worker that deletes expired record is running, returns false otherwise.
	StartingDeleteExpired() bool
}

type (
	gocache struct {
		shards
	}

	shard struct {
		m              *sync.Map
		startingWorker bool
		finishWorker   chan bool
	}

	shards []*shard

	record struct {
		expire int64
		val    interface{}
	}
)

// New returns Gocache (*gocache) instance.
func New() Gocache {
	g := &gocache{
		shards: make(shards, DefaultConcurrentMapCount),
	}

	for i := 0; i < int(DefaultConcurrentMapCount); i++ {
		g.shards[i] = &shard{
			m:              new(sync.Map),
			startingWorker: false,
			finishWorker:   make(chan bool),
		}
	}

	return g
}

func (g *gocache) Get(key string) (interface{}, bool) {
	shard := g.shards.getShard(key)

	record, ok := shard.get(key)
	if !ok {
		return nil, false
	}

	return record.val, ok
}

func (g *gocache) GetExpire(key string) (int64, bool) {
	shard := g.shards.getShard(key)

	record, ok := shard.get(key)
	if !ok {
		return 0, false
	}

	return record.expire, ok
}

func (g *gocache) Set(key string, val interface{}) bool {
	shard := g.shards.getShard(key)
	return shard.set(key, val, DefaultExpire)
}

func (g *gocache) SetWithExpire(key string, val interface{}, expire time.Duration) bool {
	shard := g.shards.getShard(key)
	return shard.set(key, val, expire)
}

func (g *gocache) Delete(key string) bool {
	shard := g.shards.getShard(key)
	return shard.delete(key)
}

func (g *gocache) DeleteExpired() {
	for _, shard := range g.shards {
		shard.deleteExpired()
	}
}

func (g *gocache) Clear() {
	for _, shard := range g.shards {
		shard.deleteAll()
	}
}

func (g *gocache) StartDeleteExpired(dur time.Duration) Gocache {
	if int(dur) <= 0 {
		return g
	}

	for _, shard := range g.shards {
		if shard.startingWorker {
			return g
		}
	}

	for _, shard := range g.shards {
		shard.startingWorker = true
		go shard.start(dur)
	}

	return g
}

func (g *gocache) StopDeleteExpired() Gocache {
	for _, shard := range g.shards {
		if shard.startingWorker {
			shard.finishWorker <- true
			shard.startingWorker = false
		}
	}

	return g
}

func (g *gocache) StartingDeleteExpired() bool {
	for _, shard := range g.shards {
		if shard.startingWorker {
			return true
		}
	}
	return false
}

func (g *record) isValid() bool {
	return fastime.Now().UnixNano() < g.expire
}

func (ss shards) getShard(key string) *shard {
	return ss[xxhash.Sum64(*(*[]byte)(unsafe.Pointer(&key)))%uint64(DefaultConcurrentMapCount)]
}

func (s *shard) get(key string) (record, bool) {
	value, ok := s.m.Load(key)
	if ok {
		i := value.(record)
		if i.isValid() {
			return i, ok
		}

		s.m.Delete(key)
	}

	return record{}, false
}

func (s *shard) set(key string, val interface{}, expire time.Duration) bool {
	if expire <= 0 {
		return false
	}

	s.m.Store(key, record{
		val:    val,
		expire: fastime.Now().Add(expire).UnixNano(),
	})

	return true
}

func (s *shard) delete(key string) bool {
	_, ok := s.m.Load(key)
	if !ok {
		return false
	}

	s.m.Delete(key)

	return true
}

func (s *shard) deleteAll() {
	s.m.Range(func(key interface{}, val interface{}) bool {
		s.m.Delete(key)
		return true
	})
}

func (s *shard) deleteExpired() {
	s.m.Range(func(key interface{}, val interface{}) bool {
		record := val.(record)

		if !record.isValid() {
			s.m.Delete(key)
		}
		return true
	})
}

func (s *shard) start(dur time.Duration) {
	t := time.NewTicker(dur)

END_LOOP:
	for {
		select {
		case _ = <-s.finishWorker:
			break END_LOOP
		case _ = <-t.C:
			s.deleteExpired()
		}
	}
	t.Stop()
}
