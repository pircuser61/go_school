package hrgate

import "time"

type Config struct {
	HRGateURL string      `yaml:"hr_gate_url"`
	Scope     string      `yaml:"scope"`
	Cache     CacheConfig `yaml:"cache"`
}

type CacheConfig struct {
	Type    string        `yaml:"type"`
	Address string        `yaml:"address"`
	DB      int           `yaml:"db"`
	Pass    string        `yaml:"pass"`
	TTL     time.Duration `yaml:"ttl"`
}
