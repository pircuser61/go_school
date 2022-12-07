package kafka

import (
	"fmt"

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

func (s *Service) Produce(ctx context.Context, message any) error {
	return s.producer.Produce(ctx, message)
}

//func (s *Service) functionReturnHandler(ctx context.Context, message RunnerInMessage) error {
//	return nil
//}

func (s *Service) StartConsumer(_ context.Context) {
	//handler := msgkit.NewMessageHandler[RunnerInMessage](s.log, s.functionReturnHandler, "return_from_function")
	//
	//go func() {
	//	if err := s.Consumer.Serve(ctx, handler); err != nil {
	//		os.Exit(-4)
	//	}
	//}()
}
