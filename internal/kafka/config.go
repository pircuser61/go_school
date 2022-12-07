package kafka

type Config struct {
	Brokers []string `yaml:"brokers"`

	ProducerTopic string `yaml:"producer_topic"`

	ConsumerGroup string `yaml:"consumer_group"`
	ConsumerTopic string `yaml:"consumer_topic"`
}
