package gocache

import "time"

const (
	// DefaultExpire is default expiration date.
	DefaultExpire = 50 * time.Second

	// DeleteExpiredInterval is the default interval at which the worker deltes all expired cache objects
	DeleteExpiredInterval = 10 * time.Second

	// DefaultShardsCount is the number of elements of concurrent map.
	DefaultShardsCount = 256
)

type config struct {
	ShardsCount           uint64
	Expire                int64
	DeleteExpiredInterval int64
}

func newDefaultConfig() *config {
	return &config{
		ShardsCount:           uint64(DefaultShardsCount),
		Expire:                int64(DefaultExpire),
		DeleteExpiredInterval: int64(DeleteExpiredInterval),
	}
}
