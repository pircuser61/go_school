package fileregistry

import "time"

type Config struct {
	REST       string        `yaml:"rest"`
	GRPC       string        `yaml:"grpc"`
	MaxRetries uint          `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
	Timeout    time.Duration `yaml:"timeout"`
}
