package gocache

import (
	"sync"
	"time"
	"unsafe"

	"github.com/cespare/xxhash"
	"github.com/kpango/fastime"
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
	Delete(string)

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
		shards shards
		*config
	}

	shard struct {
		*sync.Map
		startingWorker bool
		finishWorker   chan bool
	}

	shards []*shard

	record struct {
		val    interface{}
		expire int64
	}
)

func (g *record) isValid() bool {
	return fastime.Now().UnixNano() < g.expire
}

// New returns Gocache (*gocache) instance.
func New(options ...Option) Gocache {
	g := newDefaultGocache()

	for _, opt := range options {
		opt(g)
	}

	for i := 0; i < DefaultShardsCount; i++ {
		g.shards[i] = newDefaultShard()
	}

	return g
}

func newDefaultGocache() *gocache {
	return &gocache{
		shards: make(shards, DefaultShardsCount),
		config: newDefaultConfig(),
	}
}

func newDefaultShard() *shard {
	return &shard{
		Map:            new(sync.Map),
		startingWorker: false,
		finishWorker:   make(chan bool),
	}
}

func (g *gocache) getShard(key string) *shard {
	return g.shards[xxhash.Sum64(*(*[]byte)(unsafe.Pointer(&key)))%g.ShardsCount]
}

func (g *gocache) Get(key string) (interface{}, bool) {
	record, ok := g.getShard(key).get(key)
	if !ok {
		return nil, false
	}

	return record.val, ok
}

func (g *gocache) GetExpire(key string) (int64, bool) {
	record, ok := g.getShard(key).get(key)
	if !ok {
		return 0, false
	}

	return record.expire, ok
}

func (g *gocache) Set(key string, val interface{}) bool {
	shard := g.getShard(key)
	return shard.set(key, val, g.Expire)
}

func (g *gocache) SetWithExpire(key string, val interface{}, expire time.Duration) bool {
	shard := g.getShard(key)
	return shard.set(key, val, *(*int64)(unsafe.Pointer(&expire)))
}

func (g *gocache) Delete(key string) {
	g.getShard(key).delete(key)
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

func (s *shard) get(key string) (record, bool) {
	value, ok := s.Load(key)
	if ok {
		rcd := value.(record)
		if rcd.isValid() {
			return rcd, ok
		}

		s.Delete(key)
	}

	return record{}, false
}

func (s *shard) set(key string, val interface{}, expire int64) bool {
	if expire <= 0 {
		return false
	}

	s.Store(key, record{
		val:    val,
		expire: fastime.Now().UnixNano() + expire,
	})

	return true
}

func (s *shard) delete(key string) {
	s.Delete(key)
}

func (s *shard) deleteAll() {
	s.Range(func(key interface{}, val interface{}) bool {
		s.Delete(key)
		return true
	})
}

func (s *shard) deleteExpired() {
	s.Range(func(key interface{}, val interface{}) bool {
		record := val.(record)

		if !record.isValid() {
			s.Delete(key)
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
