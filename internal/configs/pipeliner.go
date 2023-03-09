package configs

import (
	"fmt"
	"os"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/file"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	mail_fetcher "gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/fetcher"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"

	monconf "gitlab.services.mts.ru/erius/monitoring/pkg/configs"
)

const (
	StrategyHTTP  = "http"
	StrategyKafka = "kafka"
)

type Probes struct {
	Readiness string `yaml:"readiness"`
	Liveness  string `yaml:"liveness"`
}

type Pipeliner struct {
	Tracing               TracingConfig       `yaml:"tracing"`
	Timeout               Duration            `yaml:"timeout"`
	Proxy                 string              `yaml:"proxy"`
	Log                   *logger.Config      `yaml:"log"`
	ServeAddr             string              `yaml:"serve_addr"`
	Probes                Probes              `yaml:"probes"`
	MetricsAddr           string              `yaml:"metrics_addr"`
	DB                    Database            `yaml:"database"`
	Remedy                string              `yaml:"remedy"`
	FaaS                  string              `yaml:"faas"`
	RunEnv                RunEnv              `yaml:"run_env"`
	Swag                  SwaggerGeneral      `yaml:"swagger"`
	Monitoring            monconf.Monitoring  `yaml:"monitoring"`
	AuthBaseURL           *URL                `yaml:"auth"`
	SchedulerBaseURL      *URL                `yaml:"scheduler"`
	NetworkMonitorBaseURL *URL                `yaml:"network_monitor"`
	Push                  PushConfig          `yaml:"push"`
	HTTPClientConfig      *HTTPClient         `yaml:"http_client_config"`
	SSO                   sso.Config          `yaml:"sso"`
	People                people.Config       `yaml:"people"`
	GRPCPort              string              `yaml:"grpc_gw_port"`
	GRPCGWPort            string              `yaml:"grpc_port"`
	Mail                  mail.Config         `yaml:"mail"`
	ServiceDesc           servicedesc.Config  `yaml:"servicedesc"`
	Kafka                 kafka.Config        `yaml:"kafka"`
	FunctionStore         functions.Config    `yaml:"function_store"`
	HumanTasks            human_tasks.Config  `yaml:"human_tasks"`
	MailFetcher           mail_fetcher.Config `yaml:"imap"`
	Integrations          integrations.Config `yaml:"integrations"`
	HrGate                hrgate.Config       `yaml:"hrgate"`
	Minio                 file.Config         `yaml:"minio"`
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
	UserEnvKey     string `yaml:"user_env_key"`
	PassEnvKey     string `yaml:"pass_env_key"`
	DBName         string `yaml:"dbname"`
	MaxConnections int    `yaml:"max_connections"`
	Timeout        int    `yaml:"timeout"`
}

type PushConfig struct {
	URL string `yaml:"url"`
	Job string `yaml:"job"`
}

func (d *Database) String() string {
	passPlaceholder := ""
	for range os.Getenv(d.PassEnvKey) {
		passPlaceholder += "*"
	}

	return fmt.Sprintf(
		"DB: (Kind: %s, Host: %s, Port: %s, User: %s, Pass: %s, DBName: %s, MaxConn: %d, Timeout: %d)",
		d.Kind, d.Host, d.Port, os.Getenv(d.UserEnvKey), passPlaceholder, d.DBName, d.MaxConnections, d.Timeout)
}

type HTTPClient struct {
	Timeout               Duration `yaml:"timeout"`
	KeepAlive             Duration `yaml:"keep_alive"`
	MaxIdleConns          int      `yaml:"max_idle_conns"`
	IdleConnTimeout       Duration `yaml:"idle_conn_timeout"`
	TLSHandshakeTimeout   Duration `yaml:"tls_handshake_timeout"`
	ExpectContinueTimeout Duration `yaml:"expect_continue_timeout"`
	ProxyURL              URL      `yaml:"proxy_url"`
}
