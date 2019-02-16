package gocache

import "time"

const (
	// DefaultExpire is default expiration date.
	DefaultExpire = int64(50 * time.Second)

	// DeleteExpiredInterval is the default interval at which the worker deltes all expired cache objects
	DeleteExpiredInterval = int64(10 * time.Second)

	// DefaultShardsCount is the number of elements of concurrent map.
	DefaultShardsCount uint64 = 256

	s = 1 * time.Second
)

type config struct {
	ShardsCount           uint64
	Expire                int64
	DeleteExpiredInterval int64
}

func newDefaultConfig() *config {
	return &config{
		ShardsCount:           DefaultShardsCount,
		Expire:                DefaultExpire,
		DeleteExpiredInterval: DeleteExpiredInterval,
	}
}
