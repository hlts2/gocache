package gocache

import (
	"time"

	"github.com/hlts2/lock-free"
)

const (
	defaultExpire         = 50 * time.Second
	deleteExpiredJobInval = 10 * time.Second
)

type (

	// Gocache is base interface type
	Gocache interface {
		Get(interface{}) (interface{}, bool)
		GetExpire(interface{}) (int64, bool)
		Set(interface{}, interface{}) bool
		SetWithExpire(interface{}, interface{}, time.Duration) bool
		Delete(interface{}) bool
		Clear()
		StartDeleteExpired(dur time.Duration) bool
		StopDeleteExpired() bool
	}

	gocache struct {
		lf          lockfree.LockFree
		m           map[interface{}]*value
		startingJob bool
		finishJob   chan bool
	}

	value struct {
		expire int64
		val    interface{}
	}
)

// New returns Gocache (*gocache) instance
func New() Gocache {
	g := &gocache{
		lf:          lockfree.New(),
		m:           make(map[interface{}]*value),
		startingJob: false,
		finishJob:   make(chan bool),
	}

	g.StartDeleteExpired(defaultExpire)

	return g
}

func (g *gocache) StartDeleteExpired(dur time.Duration) bool {
	if int(dur) <= 0 {
		return false
	}

	g.StopDeleteExpired()

	go g.start(dur)

	g.startingJob = true

	return true
}

func (g *gocache) StopDeleteExpired() bool {
	if g.startingJob {
		g.finishJob <- true
		g.startingJob = false
		return true
	}

	return false
}

func (g *gocache) start(dur time.Duration) {
	go func() {
		t := time.NewTicker(dur)

	END_LOOP:
		for {
			select {
			case _ = <-g.finishJob:
				break END_LOOP
			case _ = <-t.C:
				g.DeleteExpired()
			}
		}
	}()
}

func (g *value) isValid() bool {
	return time.Now().UnixNano() > g.expire
}

func (g *gocache) Get(key interface{}) (interface{}, bool) {
	defer g.lf.Signal()
	g.lf.Wait()

	value, ok := g.get(key)
	if value == nil {
		return nil, ok
	}

	return value.val, ok
}

func (g *gocache) GetExpire(key interface{}) (int64, bool) {
	defer g.lf.Signal()
	g.lf.Wait()

	value, ok := g.get(key)
	if value == nil {
		return 0, ok
	}

	return value.expire, ok
}

func (g *gocache) get(key interface{}) (*value, bool) {
	if value, ok := g.m[key]; ok {
		if value.isValid() {
			return value, ok
		}

		g.delete(key)
	}

	return nil, false
}

func (g *gocache) Set(key, val interface{}) bool {
	g.lf.Wait()

	ok := g.set(key, val, defaultExpire)

	g.lf.Signal()

	return ok
}

func (g *gocache) SetWithExpire(key, val interface{}, expire time.Duration) bool {
	g.lf.Wait()

	ok := g.set(key, val, expire)

	g.lf.Signal()

	return ok
}

func (g *gocache) set(key, val interface{}, expire time.Duration) bool {
	if expire <= 0 {
		return false
	}

	exp := time.Now().Add(expire).UnixNano()

	g.m[key] = &value{
		val:    val,
		expire: exp,
	}
	return true
}

func (g *gocache) Delete(key interface{}) bool {
	g.lf.Wait()

	ok := g.delete(key)

	g.lf.Signal()

	return ok
}

func (g *gocache) DeleteExpired() {
	for key, value := range g.m {
		g.lf.Wait()

		if !value.isValid() {
			g.delete(key)
		}

		g.lf.Signal()
	}
}

func (g *gocache) delete(key interface{}) bool {
	if _, ok := g.m[key]; !ok {
		return false
	}

	delete(g.m, key)

	return true
}

func (g *gocache) Clear() {
	g.lf.Wait()

	g.clear()

	g.lf.Signal()
}

func (g *gocache) clear() {
	g.m = make(map[interface{}]*value)
}
