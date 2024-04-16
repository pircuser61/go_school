package redisdb

import (
	"time"

	"github.com/go-redis/redis/v8"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

type DB struct {
	Cli *redis.Client

	ttlRunnerInMsg time.Duration
}

// New creates Redis client instance.
func New(cfg *Config) *DB {
	opts := &redis.Options{
		Addr: cfg.Address,
	}

	if cfg.Pass != "" {
		opts.Password = cfg.Pass
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

	err := r.Cli.Close()
	if err != nil {
		log.WithError(err).Error("Error during redis connection closure")
	}
}
