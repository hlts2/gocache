package gocache

import (
	"testing"
	"time"

	"github.com/allegro/bigcache"
	"github.com/bluele/gcache"
	"github.com/bouk/monkey"
	"github.com/kpango/gache"
	cache "github.com/patrickmn/go-cache"
)

var data = map[string]string{
	"key_1": "key_1_value",
	"key_2": "key_2_value",
	"key_3": "key_3_value",
	"key_4": "key_4_value",
}

func BenchmarkGocache(b *testing.B) {
	monkey.Unpatch(time.Now)

	g := New()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				g.Set(k, v)

				val, ok := g.Get(k)
				if !ok {
					b.Errorf("Gocache Get faild. key: %v, val: %v", k, v)
				}

				if v != val {
					b.Errorf("Gocache Get expected: %v, but got: %v", v, val)
				}
			}
		}
	})
}

func BenchmarkGache(b *testing.B) {
	g := gache.New()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				g.Set(k, v)

				val, ok := g.Get(k)
				if !ok {
					b.Errorf("Gache Get failed key: %v val: %v", k, v)
				}

				if val != v {
					b.Errorf("Gache expected %v, but got %v", v, val)
				}
			}
		}
	})
}

func BenchmarkBigCache(b *testing.B) {
	cfg := bigcache.DefaultConfig(10 * time.Minute)
	cfg.Verbose = false
	c, _ := bigcache.NewBigCache(cfg)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				c.Set(k, []byte(v))

				val, err := c.Get(k)
				if err != nil {
					b.Errorf("BigCahce Get failed key: %v val: %v", k, v)
				}

				if v != string(val) {
					b.Errorf("BigCache expected %v, but got %v", v, string(val))
				}
			}
		}
	})
}

func BenchmarkGoCache(b *testing.B) {
	c := cache.New(5*time.Minute, 10*time.Minute)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				c.Set(k, v, cache.DefaultExpiration)
				val, ok := c.Get(k)

				if !ok {
					b.Errorf("Go-Cache Get failed key: %v val: %v", k, v)
				}

				if val != v {
					b.Errorf("Go-Cache expected %v, but got %v", v, val)
				}
			}
		}
	})
}

func BenchmarkGCacheLRU(b *testing.B) {
	gc := gcache.New(20).
		LRU().
		Build()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				gc.SetWithExpire(k, v, time.Second*30)
				val, err := gc.Get(k)

				if err != nil {
					b.Errorf("GCache Get failed key: %v val: %v", k, v)
				}

				if val != v {
					b.Errorf("GCache expected %v, but got %v", v, val)
				}
			}
		}
	})
}

func BenchmarkGCacheLFU(b *testing.B) {
	gc := gcache.New(20).
		LFU().
		Build()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				gc.SetWithExpire(k, v, time.Second*30)
				val, err := gc.Get(k)

				if err != nil {
					b.Errorf("GCache Get failed key: %v val: %v", k, v)
				}

				if val != v {
					b.Errorf("GCache expected %v, but got %v", v, val)
				}
			}
		}
	})
}

func BenchmarkGCacheARC(b *testing.B) {
	gc := gcache.New(20).
		ARC().
		Build()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for k, v := range data {
				gc.SetWithExpire(k, v, time.Second*30)

				val, err := gc.Get(k)
				if err != nil {
					b.Errorf("GCache Get failed key: %v val: %v", k, v)
				}

				if val != v {
					b.Errorf("GCache expected %v, but got %v", v, val)
				}
			}
		}
	})
}
