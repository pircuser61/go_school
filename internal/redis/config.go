package redisdb

import "time"

type Config struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`

	TTLRunnerInMsg time.Duration `yaml:"ttl_runner_in_msg"`
}
