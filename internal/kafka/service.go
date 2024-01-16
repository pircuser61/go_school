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

	brokers       []string
	topics        []string
	serviceConfig Config

	MessageHandler *msgkit.MessageHandler[RunnerInMessage]
}

func NewService(log logger.Logger, cfg Config) (*Service, error) {
	topics := []string{cfg.ProducerTopic, cfg.ConsumerTopic}

	if len(cfg.Brokers) == 0 || len(topics) == 0 {
		return nil, errors.New("brokers or topics is empty")
	}

	if cfg.HealthCheckTimeout == 0 {
		return nil, errors.New("field health_check is empty")
	}

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

	return &Service{
		log: log,

		topics:        topics,
		brokers:       cfg.Brokers,
		serviceConfig: cfg,

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

// nolint:gocognit //its ok here
func (s *Service) StartCheckHealth() {
	for {
		to := time.After(s.serviceConfig.HealthCheckTimeout * time.Second)
		select {
		case <-to:
			m := metrics.DefaultRegistry
			m.UnregisterAll()

			saramaCfg := sarama.NewConfig()
			saramaCfg.MetricRegistry = m
			saramaCfg.Producer.Return.Successes = true // Producer.Return.Successes must be true to be used in a SyncProducer

			admin, err := sarama.NewClusterAdmin(s.brokers, saramaCfg)
			if err != nil {
				s.log.WithError(err).Error("error create new cluster")

				msg := s.MessageHandler

				s, err = NewService(s.log, s.serviceConfig)
				if err != nil {
					s.log.WithError(err).Error("error create new service")

					continue
				}

				s.MessageHandler = msg

				continue
			}

			topics, topicErr := admin.DescribeTopics(s.topics)
			if topicErr != nil {
				s.log.WithError(topicErr).Error("error describe topics")

				adminErr := admin.Close()
				if adminErr != nil {
					s.log.WithError(adminErr).Error("couldn't close admin client connection")
				}

				continue
			}

			for _, v := range topics {
				if v.Err == 0 {
					continue
				}

				s.log.WithError(err).Error(fmt.Sprintf("topic %s exists error", v.Name))

				adminErr := admin.Close()
				if adminErr != nil {
					s.log.WithError(adminErr).Error("couldn't close admin client connection")
				}
			}

			adminErr := admin.Close()
			if adminErr != nil {
				s.log.WithError(adminErr).Error("couldn't close admin client connection")
			}
		}
	}
}
