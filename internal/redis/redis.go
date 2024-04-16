package redisdb

import (
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

type DB struct {
	*redis.Client

	ttlRunnerInMsg time.Duration
}

// New creates Redis client instance.
func New(cfg *Config) *DB {
	opts := &redis.Options{
		Addr: cfg.Host + ":" + cfg.Port,
	}

	rdb := redis.NewClient(opts)

	return &DB{
		rdb,
		cfg.TTLRunnerInMsg,
	}
}

// Close gracefully closes all redis connections.
func (r *DB) Close() {
	log := logger.CreateLogger(nil)

	err := r.Client.Close()
	if err != nil {
		log.WithError(err).Error("Error during redis connection closure")
	}
}
