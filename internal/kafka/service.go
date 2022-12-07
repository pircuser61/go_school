package kafka

import (
	"fmt"
	"github.com/Shopify/sarama"
	"golang.org/x/net/context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	msgkit "gitlab.services.mts.ru/jocasta/msg-kit"
)

type Service struct {
	log logger.Logger

	Producer *msgkit.Producer
	Consumer *msgkit.Consumer
}

func NewService(log logger.Logger, cfg Config) (*Service, error) { //ctx context.Context, config Config) (*Service, error) {
	saramaCfg := sarama.NewConfig()
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

		Producer: producer,
		Consumer: consumer,
	}, nil
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
