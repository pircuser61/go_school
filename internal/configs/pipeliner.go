package configs

import (
	"fmt"
	"os"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/forms"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	mail_fetcher "gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/fetcher"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sequence"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type Probes struct {
	Readiness string `yaml:"readiness"`
	Liveness  string `yaml:"liveness"`
}

type Pipeliner struct {
	Tracing                 TracingConfig            `yaml:"tracing"`
	Timeout                 time.Duration            `yaml:"timeout"`
	Proxy                   string                   `yaml:"proxy"`
	Log                     *logger.Config           `yaml:"log"`
	ServeAddr               string                   `yaml:"serve_addr"`
	Probes                  Probes                   `yaml:"probes"`
	MetricsAddr             string                   `yaml:"metrics_addr"`
	DB                      Database                 `yaml:"database"`
	Remedy                  string                   `yaml:"remedy"`
	FaaS                    string                   `yaml:"faas"`
	RunEnv                  RunEnv                   `yaml:"run_env"`
	AuthBaseURL             *script.URL              `yaml:"auth"`
	SchedulerBaseURL        *script.URL              `yaml:"scheduler"`
	NetworkMonitorBaseURL   *script.URL              `yaml:"network_monitor"`
	Prometheus              metrics.PrometheusConfig `yaml:"prometheus"`
	HTTPClientConfig        *httpclient.Config       `yaml:"http_client_config"`
	SSO                     sso.Config               `yaml:"sso"`
	People                  people.Config            `yaml:"people"`
	GRPCPort                string                   `yaml:"grpc_gw_port"`
	GRPCGWPort              string                   `yaml:"grpc_port"`
	Mail                    mail.Config              `yaml:"mail"`
	ServiceDesc             servicedesc.Config       `yaml:"servicedesc"`
	Kafka                   kafka.Config             `yaml:"kafka"`
	FunctionStore           functions.Config         `yaml:"function_store"`
	HumanTasks              human_tasks.Config       `yaml:"human_tasks"`
	MailFetcher             mail_fetcher.Config      `yaml:"imap"`
	Integrations            integrations.Config      `yaml:"integrations"`
	HrGate                  hrgate.Config            `yaml:"hrgate"`
	FileRegistry            file_registry.Config     `yaml:"file_registry"`
	IncludePlaceholderBlock bool                     `yaml:"include_placeholder_block"`
	SchedulerTasks          scheduler.Config         `yaml:"scheduler_tasks"`
	Forms                   forms.Config             `yaml:"forms"`
	Sequence                sequence.Config          `yaml:"sequence"`
	HostURL                 string                   `yaml:"host_url"`
	LogIndex                string                   `yaml:"log_index"`
	ServicesPing            ServicesPing             `yaml:"services_ping"`
	ConsumerFuncsWorkers    int                      `yaml:"consumer_funcs_workers"`
	ConsumerTasksWorkers    int                      `yaml:"consumer_tasks_workers"`
	Retry                   RetryConfig              `yaml:"retry"`
}

type RetryConfig struct {
	MinLifetime time.Duration `yaml:"min_lifetime"`
	MaxLifetime time.Duration `yaml:"max_lifetime"`
}

type RunEnv struct {
	Strategy          string `yaml:"strategy"`
	FaaSAddress       string `yaml:"faas_address,omitempty"`
	KafkaAddress      string `yaml:"kafka_address,omitempty"`
	PipelinesRunQueue string `yaml:"pipelines_run_queue,omitempty"`
	FunctionsRunQueue string `yaml:"functions_run_queue,omitempty"`
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

type ServicesPing struct {
	PingTimer    time.Duration `yaml:"ping_timer"`
	MaxFailedCnt int           `yaml:"max_failed_count"`
	MaxOkCnt     int           `yaml:"max_ok_count"`
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
