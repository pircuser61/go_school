package integrations

import "time"

type Config struct {
	URL        string        `yaml:"url"`
	MaxRetries uint          `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
	Timeout    time.Duration `yaml:"timeout"`
}
