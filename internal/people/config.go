package people

import "time"

type Config struct {
	URL        string        `yaml:"url"`
	MaxRetries uint          `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`

	Cache CacheConfig `yaml:"cache"`
}

type CacheConfig struct {
	Type    string        `yaml:"type"`
	Address string        `yaml:"address"`
	DB      int           `yaml:"db"`
	Pass    string        `yaml:"pass"`
	TTL     time.Duration `yaml:"ttl"`
}
