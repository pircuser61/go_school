package configs

import "gitlab.services.mts.ru/libs/logger"

type Pipeliner struct {
	Tracing     TracingConfig  `yaml:"tracing"`
	Timeout     Duration       `yaml:"timeout"`
	Proxy       string         `yaml:"proxy"`
	Log         *logger.Config `yaml:"log"`
	ServeAddr   string         `yaml:"serve_addr"`
	MetricsAddr string         `yaml:"metrics_addr"`
	DB          Database       `yaml:"database"`
}

type Database struct {
	Kind           string `yaml:"kind"`
	Host           string `yaml:"host"`
	Port           string `yaml:"port"`
	User           string `yaml:"user"`
	Pass           string `yaml:"pass"`
	DBName         string `yaml:"dbname"`
	MaxConnections int    `yaml:"max_connections"`
	Timeout        int    `yaml:"timeout"`
}
