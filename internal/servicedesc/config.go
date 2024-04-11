package servicedesc

import "time"

type Config struct {
	ServicedeskURL string      `yaml:"servicedesk_url"`
	Scope          string      `yaml:"scope"`
	Cache          CacheConfig `yaml:"cache"`
}

type CacheConfig struct {
	Type    string        `yaml:"type"`
	Address string        `yaml:"address"`
	DB      int           `yaml:"db"`
	Pass    string        `yaml:"pass"`
	TTL     time.Duration `yaml:"ttl"`
}
