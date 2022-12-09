package kafka

import (
	"fmt"
	"os"

	"golang.org/x/net/context"

	"github.com/Shopify/sarama"

	"github.com/rcrowley/go-metrics"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	msgkit "gitlab.services.mts.ru/jocasta/msg-kit"
)

type Service struct {
	log logger.Logger

	producer *msgkit.Producer
	consumer *msgkit.Consumer

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

	return &Service{
		log: log,

		producer: producer,
		consumer: consumer,
	}, nil
}

func (s *Service) Produce(ctx context.Context, message RunnerOutMessage) error {
	return s.producer.Produce(ctx, message)
}

func (s *Service) CloseProducer() error {
	return s.producer.Close()
}

func (s *Service) InitMessageHandler(handler func(context.Context, RunnerInMessage) error) {
	s.MessageHandler = msgkit.NewMessageHandler[RunnerInMessage](s.log, handler, "function_return")
}

func (s *Service) StartConsumer(ctx context.Context) {
	go func() {
		err := s.consumer.Serve(ctx, s.MessageHandler)
		if err != nil {
			os.Exit(-4)
		}
	}()
}
