package kafka

import "time"

type Config struct {
	Brokers []string `yaml:"brokers"`

	ProducerTopic   string `yaml:"producer_topic"`
	ProducerTopicSD string `yaml:"producer_topic_sd"`

	ConsumerGroupFunctions  string `yaml:"consumer_group_functions"`
	ConsumerGroupTaskRunner string `yaml:"consumer_group_task_runner"`
	ConsumerFunctionsTopic  string `yaml:"consumer_functions_topic"`
	ConsumerTaskRunnerTopic string `yaml:"consumer_task_runner_topic"`

	HealthCheckTimeout     int           `yaml:"health_check_timeout"`
	FuncMessageResendDelay time.Duration `yaml:"function_message_resend_delay"`
}
