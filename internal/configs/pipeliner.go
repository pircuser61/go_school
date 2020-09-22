package configs

import (
	"fmt"

	"gitlab.services.mts.ru/libs/logger"
)

const (
	StrategyHttp  = "http"
	StrategyKafka = "kafka"
)

type Pipeliner struct {
	Tracing       TracingConfig  `yaml:"tracing"`
	Timeout       Duration       `yaml:"timeout"`
	Proxy         string         `yaml:"proxy"`
	Log           *logger.Config `yaml:"log"`
	ServeAddr     string         `yaml:"serve_addr"`
	MetricsAddr   string         `yaml:"metrics_addr"`
	DB            Database       `yaml:"database"`
	ScriptManager string         `yaml:"script_manager"`
	FaaS          string         `yaml:"faas"`
	RunEnv        RunEnv         `yaml:"run_env"`
	Swag          SwaggerGeneral `yaml:"swagger"`
	Monitoring    Monitoring     `yaml:"monitoring"`
	AuthBaseURL   *URL           `yaml:"auth"`
	Push          PushConfig     `yaml:"push"`
}

type RunEnv struct {
	Strategy          string `yaml:"strategy"`
	FaaSAddress       string `yaml:"faas_address,omitempty"`
	KafkaAddress      string `yaml:"kafka_address,omitempty"`
	PipelinesRunQueue string `yaml:"pipelines_run_queue,omitempty"`
	FunctionsRunQueue string `yaml:"functions_run_queue,omitempty"`
}

type SwaggerGeneral struct {
	BasePath string `yaml:"base_path"`
	Version  string `yaml:"version"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
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

type Monitoring struct {
	Addr    string `yaml:"addr"`
	Timeout int    `yaml:"timeout"`
}

type PushConfig struct {
	URL string `yaml:"url"`
	Job string `yaml:"job"`
}

func (d *Database) String() string {
	pass := ""
	for range d.Pass {
		pass += "*"
	}

	return fmt.Sprintf(
		"DB: (Kind: %s, Host: %s, Port: %s, User: %s, Pass: %s, DBName: %s, MaxConn: %d, Timeout: %d)",
		d.Kind, d.Host, d.Port, d.User, pass, d.DBName, d.MaxConnections, d.Timeout)
}
