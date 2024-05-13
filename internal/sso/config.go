package sso

import "time"

type Config struct {
	Address               string        `yaml:"address"`
	ClientSecretEnvKey    string        `yaml:"client_secret_env_key"`
	ClientID              string        `yaml:"client_id"`
	Realm                 string        `yaml:"realm"`
	AccessTokenCookieName string        `yaml:"access_cookie_name"`
	MaxRetries            uint          `yaml:"max_retries"`
	RetryDelay            time.Duration `yaml:"retry_delay"`
}
