package kafka

import "time"

type Config struct {
	Brokers []string `yaml:"brokers"`

	ProducerTopic string `yaml:"producer_topic"`

	ConsumerGroup string `yaml:"consumer_group"`
	ConsumerTopic string `yaml:"consumer_topic"`

	Delay time.Duration `yaml:"delay"`
}
