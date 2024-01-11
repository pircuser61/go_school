package kafka

import (
	c "context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"github.com/rcrowley/go-metrics"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	msgkit "gitlab.services.mts.ru/jocasta/msg-kit"
)

type Service struct {
	log logger.Logger

	producer *msgkit.Producer
	consumer *msgkit.Consumer

	brokers     []string
	topics      []string
	config      *sarama.Config
	configKafka Config

	MessageHandler *msgkit.MessageHandler[RunnerInMessage]
}

func NewService(log logger.Logger, cfg Config) (*Service, error) {
	m := metrics.DefaultRegistry
	m.UnregisterAll()
	saramaCfg := sarama.NewConfig()
	saramaCfg.MetricRegistry = m
	saramaCfg.Producer.Return.Successes = true // Producer.Return.Successes must be true to be used in a SyncProducer

	saramaClient, err := sarama.NewClient(cfg.Brokers, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	producer, err := msgkit.NewProducer(saramaClient, cfg.ProducerTopic)
	if err != nil {
		return nil, err
	}

	consumer, err := msgkit.NewConsumer(saramaClient, cfg.ConsumerGroup, cfg.ConsumerTopic)
	if err != nil {
		return nil, err
	}

	topics := []string{cfg.ProducerTopic, cfg.ConsumerTopic}

	return &Service{
		log: log,

		topics:      topics,
		brokers:     cfg.Brokers,
		config:      saramaCfg,
		configKafka: cfg,

		producer: producer,
		consumer: consumer,
	}, nil
}

func (s *Service) Produce(ctx c.Context, message RunnerOutMessage) error {
	if s == nil {
		return errors.New("kafka service unavailable")
	}

	return s.producer.Produce(ctx, message)
}

func (s *Service) CloseProducer() error {
	if s != nil {
		return s.producer.Close()
	}

	return nil
}

func (s *Service) InitMessageHandler(handler func(c.Context, RunnerInMessage) error) {
	if s == nil {
		return
	}

	s.MessageHandler = msgkit.NewMessageHandler[RunnerInMessage](s.log, handler, "function_return")
}

func (s *Service) StartConsumer(ctx c.Context) {
	if s == nil {
		return
	}

	go func() {
		err := s.consumer.Serve(ctx, s.MessageHandler)
		if err != nil {
			s.log.Error(err)
			os.Exit(-4)
		}
	}()
}

func (s *Service) StartCheckHealth() error {
	if len(s.brokers) == 0 || len(s.topics) == 0 {
		return errors.New("brokers or topics is emptys")
	}

	chanErr := make(chan error, 1)
	go func() {
		for {
			time.Sleep(30 * time.Second)
			admin, err := sarama.NewClusterAdmin(s.brokers, s.config)
			if err != nil {
				s.log.Error(err)
				chanErr <- err
			}
			defer admin.Close()

			select {
			case <-chanErr:
				msg := s.MessageHandler

				s, err = NewService(s.log, s.configKafka)
				if err != nil {
					s.log.Error(err)
					chanErr <- err
					continue
				}

				s.MessageHandler = msg
			default:
				topics, topicErr := admin.DescribeTopics(s.topics)
				if topicErr == nil {
					for _, v := range topics {
						if v.Err != 0 {
							s.log.Error("topic error ", v.Err)
							chanErr <- v.Err
							continue
						}
					}
					continue
				}

				s.log.Error("error describe topics: ", err)
				chanErr <- err
			}
		}
	}()

	return nil
}
