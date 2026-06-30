package gocache

import (
	"math/bits"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/kpango/fastime"
)

// Gocache is base gocache interface.
type Gocache interface {

	// Get returns object with the given name from the cache.
	Get(string) (interface{}, bool)

	// Set sets object in the cache.
	Set(string, interface{}) bool

	// SetWithExpire sets object in cache with an expiration date.
	SetWithExpire(string, interface{}, time.Duration) bool

	// Delete deletes cache object of given name.
	Delete(string)

	// Clear clears cache.
	Clear()

	// StartExpired starts worker that deletes an expired cache object.
	// Deletion processing is executed at intervals of given time.
	StartExpired(dur time.Duration) Gocache

	// StopExpired stop worker that deletes an expired cache object.
	StopExpired() Gocache
}

const (
	// initialTableSize is the number of bucket slots a shard table starts with.
	initialTableSize uint64 = 64

	hashK1 uint64 = 0xa0761d6478bd642f
	hashK2 uint64 = 0xe7037ed1a0b428db
	hashK3 uint64 = 0x8ebc6af09c88c6e3
	hashK4 uint64 = 0x589965cc75374cc3
)

type (
	gocache struct {
		shards []shard
		mask   uint64
		*config
		mu     sync.Mutex
		doneCh chan struct{}
	}

	// shard is a fixed slice of the key space. Reads and same-key writes
	// never touch shard.mu; it only guards new-key inserts and table swaps.
	shard struct {
		mu   sync.Mutex
		tab  atomic.Pointer[table]
		used uint64
		_    [40]byte // pad to a cache line to avoid false sharing
	}

	// table is an open-addressing (linear probing) hash table. A published
	// table is immutable except for the per-slot atomic entry pointers,
	// which only ever go nil -> non-nil; growth publishes a new table.
	// first caches the backing-array pointer so hot paths can probe
	// without slice-header loads or bounds checks.
	table struct {
		mask    uint64
		first   *atomic.Pointer[entry]
		buckets []atomic.Pointer[entry]
	}

	// entry is a single cache slot. The current value is swapped in as an
	// immutable boxed interface with a single atomic store, so Get and
	// overwrite-Set are lock-free and wait-free. expire lives next to it
	// and is skipped on store when unchanged; a Get may transiently pair a
	// value with a neighboring Set's expiry, which is benign for cache
	// semantics. Padded to a cache line so hot entries don't false-share.
	entry struct {
		hash   uint64
		key    string
		prefix uint64
		rec    atomic.Pointer[interface{}]
		expire atomic.Int64
		_      [16]byte
	}
)

// New returns Gocache (*gocache) instance.
func New(options ...Option) Gocache {
	g := &gocache{
		config: newDefaultConfig(),
	}

	for _, opt := range options {
		opt(g)
	}

	n := nextPow2(g.ShardsCount)
	g.shards = make([]shard, n)
	g.mask = n - 1
	for i := range g.shards {
		g.shards[i].tab.Store(newTable(initialTableSize))
	}

	return g
}

func nextPow2(n uint64) uint64 {
	if n < 2 {
		return 1
	}
	v := uint64(1)
	for v < n {
		v <<= 1
	}
	return v
}

func newTable(size uint64) *table {
	b := make([]atomic.Pointer[entry], size)
	return &table{
		mask:    size - 1,
		first:   &b[0],
		buckets: b,
	}
}

// slot returns the i-th bucket slot without a bounds check; i must
// already be masked by the caller.
func (t *table) slot(i uint64) *atomic.Pointer[entry] {
	return (*atomic.Pointer[entry])(unsafe.Add(unsafe.Pointer(t.first), uintptr(i)*unsafe.Sizeof(*t.first)))
}

// shard returns the shard owning hash h without a bounds check.
func (g *gocache) shard(h uint64) *shard {
	return (*shard)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(g.shards)), uintptr(h&g.mask)*unsafe.Sizeof(shard{})))
}

// hashKey returns a wyhash-style hash of key (8-byte loads folded
// through 128-bit multiplies, with overlapping reads for short tails)
// plus an encoding of key that, together with its length, uniquely
// identifies any key of up to 8 bytes (short == true).
func hashKey(key string) (uint64, uint64, bool) {
	p := unsafe.Pointer(unsafe.StringData(key))
	l := uintptr(len(key))
	seed := hashK1
	var a, b, pfx uint64
	short := false
	switch {
	case l > 64:
		if l > 384 {
			// very long keys are hashed by sampling 32 bytes each from
			// the head, middle and tail (plus length), touching as few
			// cache lines as possible; the tail sample adjoins the
			// 16-byte tail read below. A difference outside the samples
			// only costs extra probe collisions; lookups always verify
			// full key equality.
			seed = hash64(p, 0, seed)
			seed = hash64(p, l/2-32, seed)
			seed = hash64(p, l-80, seed)
		} else {
			// four independent lanes keep the multiplier pipeline full
			// on long keys instead of one latency-bound chain.
			s1, s2, s3 := seed, seed, seed
			i := uintptr(0)
			for ; i+64 < l; i += 64 {
				seed = mix(load64(p, i)^hashK2, load64(p, i+8)^seed)
				s1 = mix(load64(p, i+16)^hashK3, load64(p, i+24)^s1)
				s2 = mix(load64(p, i+32)^hashK4, load64(p, i+40)^s2)
				s3 = mix(load64(p, i+48)^hashK2, load64(p, i+56)^s3)
			}
			seed ^= mix(s1^hashK3, s2^hashK4) ^ s3
			for ; i+16 < l; i += 16 {
				seed = mix(load64(p, i)^hashK2, load64(p, i+8)^seed)
			}
		}
		a = load64(p, l-16)
		b = load64(p, l-8)
	case l > 16:
		i := uintptr(0)
		for ; i+16 < l; i += 16 {
			seed = mix(load64(p, i)^hashK2, load64(p, i+8)^seed)
		}
		a = load64(p, l-16)
		b = load64(p, l-8)
	case l >= 8:
		a = load64(p, 0)
		b = load64(p, l-8)
		if l == 8 {
			pfx, short = a, true
		}
	case l >= 4:
		a = uint64(load32(p, 0))
		b = uint64(load32(p, l-4))
		pfx, short = a|b<<32, true
	case l > 0:
		a = uint64(*(*byte)(p))<<16 |
			uint64(*(*byte)(unsafe.Add(p, l>>1)))<<8 |
			uint64(*(*byte)(unsafe.Add(p, l-1)))
		pfx, short = a, true
	default:
		short = true
	}
	return mix(a^seed^uint64(l), b^hashK2), pfx, short
}

// stringType is the runtime type descriptor of string, used to
// recognize string values for the identical-value store skip.
var stringType = func() unsafe.Pointer {
	var v interface{} = ""
	return (*eface)(unsafe.Pointer(&v)).typ
}()

// eface mirrors the runtime layout of an empty interface.
type eface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

// sameValue reports whether the boxed values a and b are identical:
// the same box, or string headers naming the same bytes. It never
// inspects value contents, so it is safe for any type.
func sameValue(a, b *interface{}) bool {
	ea := (*eface)(unsafe.Pointer(a))
	eb := (*eface)(unsafe.Pointer(b))
	if ea.typ != eb.typ {
		return false
	}
	if ea.data == eb.data {
		return true
	}
	if ea.typ == stringType {
		sa := (*string)(ea.data)
		sb := (*string)(eb.data)
		return len(*sa) == len(*sb) && unsafe.StringData(*sa) == unsafe.StringData(*sb)
	}
	return false
}

// hash64 folds exactly 64 bytes at p+off into seed using four
// independent multiplies.
func hash64(p unsafe.Pointer, off uintptr, seed uint64) uint64 {
	s0 := mix(load64(p, off)^hashK2, load64(p, off+8)^seed)
	s1 := mix(load64(p, off+16)^hashK3, load64(p, off+24)^hashK1)
	s2 := mix(load64(p, off+32)^hashK4, load64(p, off+40)^hashK1)
	s3 := mix(load64(p, off+48)^hashK2, load64(p, off+56)^hashK1)
	return mix(s0^s1, s2^s3)
}

func mix(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return hi ^ lo
}

func load64(p unsafe.Pointer, off uintptr) uint64 {
	return *(*uint64)(unsafe.Add(p, off))
}

func load32(p unsafe.Pointer, off uintptr) uint32 {
	return *(*uint32)(unsafe.Add(p, off))
}

// longShard picks the shard for keys longer than 384 bytes from the
// head bytes and length only, so the shard/table pointer chase can
// overlap the sampled hash reads that follow. Bucket placement still
// uses the full hash.
func (g *gocache) longShard(key string) *shard {
	p := unsafe.Pointer(unsafe.StringData(key))
	return g.shard(mix(load64(p, 0)^hashK2, uint64(len(key))^hashK1))
}

func (g *gocache) Get(key string) (interface{}, bool) {
	if len(key) > 384 {
		return g.getLong(key)
	}
	h, pfx, short := hashKey(key)
	t := g.shard(h).tab.Load()
	m := t.mask
	for i := (h >> 32) & m; ; i = (i + 1) & m {
		e := t.slot(i).Load()
		if e == nil {
			return nil, false
		}
		if e.hash == h && keyEqual(e, key, pfx, short) {
			r := e.rec.Load()
			if r == nil || fastime.UnixNanoNow() >= e.expire.Load() {
				return nil, false
			}
			return *r, true
		}
	}
}

// keyEqual reports whether e holds key. For keys up to 8 bytes the
// injective prefix encoding plus length is exact, so the key bytes are
// never touched.
func keyEqual(e *entry, key string, pfx uint64, short bool) bool {
	if short {
		return e.prefix == pfx && len(e.key) == len(key)
	}
	return e.key == key
}

func (g *gocache) getLong(key string) (interface{}, bool) {
	t := g.longShard(key).tab.Load()
	h, _, _ := hashKey(key)
	m := t.mask
	for i := (h >> 32) & m; ; i = (i + 1) & m {
		e := t.slot(i).Load()
		if e == nil {
			return nil, false
		}
		if e.hash == h && e.key == key {
			r := e.rec.Load()
			if r == nil || fastime.UnixNanoNow() >= e.expire.Load() {
				return nil, false
			}
			return *r, true
		}
	}
}

func (g *gocache) Set(key string, val interface{}) bool {
	return g.set(key, val, g.Expire)
}

func (g *gocache) SetWithExpire(key string, val interface{}, expire time.Duration) bool {
	return g.set(key, val, int64(expire))
}

func (g *gocache) set(key string, val interface{}, expire int64) bool {
	if expire <= 0 {
		return false
	}

	var s *shard
	var h, pfx uint64
	var short bool
	if len(key) > 384 {
		s = g.longShard(key)
		s.tab.Load()
		h, pfx, short = hashKey(key)
	} else {
		h, pfx, short = hashKey(key)
		s = g.shard(h)
	}
	exp := fastime.UnixNanoNow() + expire

	t := s.tab.Load()
	m := t.mask
	for i := (h >> 32) & m; ; i = (i + 1) & m {
		e := t.slot(i).Load()
		if e == nil {
			break
		}
		if e.hash == h && keyEqual(e, key, pfx, short) {
			if e.expire.Load() != exp {
				e.expire.Store(exp)
			}
			if r := e.rec.Load(); r != nil && sameValue(r, &val) {
				// identical value already cached; expiry refreshed above
				return true
			}
			v := val
			e.rec.Store(&v)
			return true
		}
	}

	v := val
	s.insert(h, pfx, key, &v, exp)
	return true
}

func (g *gocache) Delete(key string) {
	var t *table
	var h, pfx uint64
	var short bool
	if len(key) > 384 {
		t = g.longShard(key).tab.Load()
		h, pfx, short = hashKey(key)
	} else {
		h, pfx, short = hashKey(key)
		t = g.shard(h).tab.Load()
	}
	m := t.mask
	for i := (h >> 32) & m; ; i = (i + 1) & m {
		e := t.slot(i).Load()
		if e == nil {
			return
		}
		if e.hash == h && keyEqual(e, key, pfx, short) {
			e.rec.Store(nil)
			return
		}
	}
}

func (g *gocache) DeleteExpired() {
	now := fastime.UnixNanoNow()
	for i := range g.shards {
		t := g.shards[i].tab.Load()
		for j := range t.buckets {
			e := t.buckets[j].Load()
			if e == nil {
				continue
			}
			if r := e.rec.Load(); r != nil && now >= e.expire.Load() {
				// drop only the value we judged expired, never a newer one
				e.rec.CompareAndSwap(r, nil)
			}
		}
	}
}

func (g *gocache) Clear() {
	for i := range g.shards {
		s := &g.shards[i]
		s.mu.Lock()
		s.tab.Store(newTable(initialTableSize))
		s.used = 0
		s.mu.Unlock()
	}
}

func (g *gocache) StartExpired(dur time.Duration) Gocache {
	if dur <= 0 {
		return g
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.doneCh == nil {
		g.doneCh = make(chan struct{})
		go g.expireLoop(dur, g.doneCh)
	}

	return g
}

func (g *gocache) StopExpired() Gocache {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.doneCh != nil {
		close(g.doneCh)
		g.doneCh = nil
	}

	return g
}

func (g *gocache) expireLoop(dur time.Duration, done chan struct{}) {
	t := time.NewTicker(dur)
	defer t.Stop()

	for {
		select {
		case <-done:
			return
		case <-t.C:
			g.DeleteExpired()
		}
	}
}

// insert stores a key that is not present in the shard's published table.
func (s *shard) insert(h, pfx uint64, key string, r *interface{}, exp int64) {
	s.mu.Lock()
	t := s.tab.Load()
	if (s.used+1)*4 > uint64(len(t.buckets))*3 {
		t = s.grow(t)
	}
	m := t.mask
	for i := (h >> 32) & m; ; i = (i + 1) & m {
		e := t.buckets[i].Load()
		if e == nil {
			ne := &entry{hash: h, key: key, prefix: pfx}
			ne.expire.Store(exp)
			ne.rec.Store(r)
			t.buckets[i].Store(ne)
			s.used++
			s.mu.Unlock()
			return
		}
		if e.hash == h && e.prefix == pfx && e.key == key {
			// another goroutine inserted the key first
			s.mu.Unlock()
			e.expire.Store(exp)
			e.rec.Store(r)
			return
		}
	}
}

// grow publishes a table twice the size, carrying over every entry pointer
// so in-flight writes against the old table remain visible. Caller must
// hold s.mu.
func (s *shard) grow(t *table) *table {
	nt := newTable((t.mask + 1) * 2)
	m := nt.mask
	for i := range t.buckets {
		e := t.buckets[i].Load()
		if e == nil {
			continue
		}
		for j := (e.hash >> 32) & m; ; j = (j + 1) & m {
			if nt.buckets[j].Load() == nil {
				nt.buckets[j].Store(e)
				break
			}
		}
	}
	s.tab.Store(nt)
	return nt
}
