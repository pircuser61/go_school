package kafka

import "time"

type Config struct {
	Brokers []string `yaml:"brokers"`

	ProducerTopic   string `yaml:"producer_topic"`
	ProducerTopicSD string `yaml:"producer_topic_sd"`

	ConsumerGroup string `yaml:"consumer_group"`
	ConsumerTopic string `yaml:"consumer_topic"`

	HealthCheckTimeout     int           `yaml:"health_check_timeout"`
	FuncMessageResendDelay time.Duration `yaml:"function_message_resend_delay"`
}
