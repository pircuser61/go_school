package redisdb

import "time"

type Config struct {
	Address string `yaml:"address"`
	Pass    string `yaml:"pass"`

	TTLRunnerInMsg time.Duration `yaml:"ttl_runner_in_msg"`
}
