package configs

import "gitlab.services.mts.ru/libs/logger"

type Manager struct {
	Tracing     TracingConfig  `yaml:"tracing"`
	Timeout     Duration       `yaml:"timeout"`
	Proxy       string         `yaml:"proxy"`
	Log         *logger.Config `yaml:"log"`
	ServeAddr   string         `yaml:"serve_addr"`
	MetricsAddr string         `yaml:"metrics_addr"`
}
