package gocache

import (
	"time"

	"github.com/hlts2/lock-free"
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
	}

	gocache struct {
		lf     lockfree.LockFree
		m      map[interface{}]*value
		expire time.Duration
	}

	value struct {
		expire int64
		val    interface{}
	}
)

// New returns Gocache (*gocache) instance
func New() Gocache {
	g := &gocache{
		lf:     lockfree.New(),
		m:      make(map[interface{}]*value),
		expire: time.Second * 50,
	}

	g.start()

	return g
}

func (g *gocache) start() Gocache {
	go func() {
		t := time.NewTicker(10 * time.Second)
		for {
			select {
			case _ = <-t.C:
				g.DeleteExpired()
			}
		}
	}()

	return g
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

	ok := g.set(key, val, g.expire)

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
