package gocache

import (
	"testing"
	"time"

	"github.com/bouk/monkey"
	"github.com/patrickmn/go-cache"
)

var data = map[string]string{
	"key_1": "key_1_value",
}

func BenchmarkGocache(b *testing.B) {
	monkey.Unpatch(time.Now)
	g := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for key, val := range data {
			g.Set(key, val)
			g.Get(key)
		}
	}
}

func BenchmarkGo_cache(b *testing.B) {
	monkey.Unpatch(time.Now)
	c := cache.New(5*time.Minute, 10*time.Minute)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for key, val := range data {
			c.Set(key, val, time.Second*50)
			c.Get(key)
		}
	}
}
