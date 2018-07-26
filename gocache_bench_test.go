package gocache

import (
	"testing"
	"time"

	"github.com/bouk/monkey"
	"github.com/patrickmn/go-cache"
)

func BenchmarkGocache(b *testing.B) {
	monkey.Unpatch(time.Now)
	g := New()

	key, val := "key_1", "key_1_value"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Set(key, val)
		g.Get(key)
	}
}

func BenchmarkGo_cache(b *testing.B) {
	monkey.Unpatch(time.Now)
	c := cache.New(5*time.Minute, 10*time.Minute)

	key, val := "key_1", "key_1_value"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.Set(key, val, 50*time.Second)
		c.Get(key)
	}
}
