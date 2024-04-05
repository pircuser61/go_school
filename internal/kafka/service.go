package kafka

import (
	c "context"
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"

	"github.com/rcrowley/go-metrics"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	msgkit "gitlab.services.mts.ru/jocasta/msg-kit"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type Service struct {
	log logger.Logger

	producerSd *msgkit.Producer
	producer   *msgkit.Producer
	consumer   *msgkit.Consumer

	brokers       []string
	topics        []string
	serviceConfig Config

	MessageHandler *msgkit.MessageHandler[RunnerInMessage]
}

const (
	kafkaNetTimeout = 3 * time.Second
)

//nolint:gocritic //если тут удобно по значению значит пусть будет по значению
func NewService(log logger.Logger, cfg Config) (*Service, bool, error) {
	s := &Service{
		log: log,

		brokers:       cfg.Brokers,
		serviceConfig: cfg,
	}

	topics := []string{cfg.ProducerTopic, cfg.ProducerTopicSD, cfg.ConsumerTopic}

	if len(cfg.Brokers) == 0 || len(topics) == 0 {
		return s, false, errors.New("brokers or topics is empty")
	}

	if cfg.HealthCheckTimeout == 0 {
		return s, false, errors.New("field health_check is empty")
	}

	s.topics = topics

	m := metrics.DefaultRegistry
	m.UnregisterAll()

	saramaCfg := sarama.NewConfig()
	saramaCfg.MetricRegistry = m
	saramaCfg.Producer.Return.Successes = true // Producer.Return.Successes must be true to be used in a SyncProducer
	saramaCfg.Net.DialTimeout = kafkaNetTimeout

	saramaClient, err := sarama.NewClient(cfg.Brokers, saramaCfg)
	if err != nil {
		return s, true, fmt.Errorf("failed to create client: %w", err)
	}

	producerToSD, err := msgkit.NewProducer(saramaClient, cfg.ProducerTopicSD)
	if err != nil {
		return s, true, err
	}

	s.producerSd = producerToSD

	producer, err := msgkit.NewProducer(saramaClient, cfg.ProducerTopic)
	if err != nil {
		return s, true, err
	}

	s.producer = producer

	consumer, err := msgkit.NewConsumer(saramaClient, cfg.ConsumerGroup, cfg.ConsumerTopic)
	if err != nil {
		return s, true, err
	}

	s.consumer = consumer

	return s, true, nil
}

func (s *Service) ProduceFuncMessage(ctx c.Context, message *RunnerOutMessage) error {
	if s == nil || s.producer == nil {
		return errors.New("kafka service unavailable")
	}

	return s.producer.Produce(ctx, message)
}

//nolint:all //its ok here
func (s *Service) ProduceEventMessage(ctx c.Context, message *e.NodeKafkaEvent) error {
	if message == nil {
		return nil
	}

	l := s.log.WithField("workNumber", message.WorkNumber).
		WithField("nodeName", message.NodeName).
		WithField("action", message.Action)

	l.Info("try to send event to kafka")

	if s == nil || s.producerSd == nil {
		return errors.New("kafka service unavailable")
	}

	err := s.producerSd.Produce(ctx, message)
	if err != nil {
		l.Error("error send event to kafka", err)

		return err
	}

	l.Info("success send event to kafka")

	return nil
}

func (s *Service) CloseProducer() error {
	if s != nil && s.producer != nil {
		err := s.producer.Close()
		if err != nil {
			return err
		}
	}

	if s != nil && s.producerSd != nil {
		err := s.producerSd.Close()
		if err != nil {
			return err
		}
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
	if s == nil || s.consumer == nil {
		return
	}

	go func() {
		err := s.consumer.Serve(ctx, s.MessageHandler)
		if err != nil {
			s.consumer = nil
			s.log.Error(err)
		}
	}()
}

// nolint:gocognit //its ok here
func (s *Service) StartCheckHealth() {
	for {
		<-time.After(time.Duration(s.serviceConfig.HealthCheckTimeout) * time.Second)

		s.checkHealth()
	}
}

func (s *Service) checkHealth() {
	m := metrics.DefaultRegistry
	m.UnregisterAll()

	saramaCfg := sarama.NewConfig()
	saramaCfg.MetricRegistry = m
	saramaCfg.Producer.Return.Successes = true // Producer.Return.Successes must be true to be used in a SyncProducer
	saramaCfg.Net.DialTimeout = kafkaNetTimeout

	admin, err := sarama.NewClusterAdmin(s.brokers, saramaCfg)
	if err != nil || s.consumer == nil || s.producer == nil {
		s.log.WithError(err).Error("couldn't connect to kafka! Trying to reconnect")

		msg := s.MessageHandler

		newService, _, reconnectErr := NewService(s.log, s.serviceConfig)
		*s = *newService
		s.MessageHandler = msg

		if reconnectErr != nil {
			s.log.WithError(reconnectErr).Error("failed to reconnect to kafka")

			return
		}

		s.StartConsumer(c.Background())

		s.log.Info("the reconnection to kafka was successful")

		return
	}

	topics, topicErr := admin.DescribeTopics(s.topics)
	if topicErr != nil {
		s.log.WithError(topicErr).Error("error describe topics")

		adminErr := admin.Close()
		if adminErr != nil {
			s.log.WithError(adminErr).Error("couldn't close admin client connection")
		}

		return
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
