package kafka

import (
	c "context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"go.opencensus.io/trace"

	gometrics "github.com/rcrowley/go-metrics"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	msgkit "gitlab.services.mts.ru/jocasta/msg-kit"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

type Service struct {
	log     logger.Logger
	cache   cachekit.Cache
	metrics metrics.Metrics

	producerSd         *msgkit.Producer
	producerFuncResult *msgkit.Producer
	producer           *msgkit.Producer
	consumerFunctions  *msgkit.Consumer
	consumerTaskRunner *msgkit.Consumer

	brokers       []string
	topics        []string
	serviceConfig Config

	FuncMessageHandler       *msgkit.MessageHandler[RunnerInMessage]
	TaskRunnerMessageHandler *msgkit.MessageHandler[RunTaskMessage]

	ctxCancel     c.CancelFunc
	isConsuming   bool
	stoppedByPing bool
}

const (
	kafkaNetTimeout = 3 * time.Second
)

//nolint:gocritic //если тут удобно по значению значит пусть будет по значению
func NewService(log logger.Logger, cfg Config, m metrics.Metrics) (*Service, bool, error) {
	s := &Service{
		log:     log,
		metrics: m,

		brokers:       cfg.Brokers,
		serviceConfig: cfg,
		stoppedByPing: false,
	}

	kafkaCache, err := cachekit.CreateCache(cachekit.Config{
		Type:    cfg.Cache.Type,
		Address: cfg.Cache.Address,
		DB:      s.getCacheDBIdx(&cfg),
		Pass:    cfg.Cache.Pass,
		TTL:     cfg.Cache.TTL,
	})
	if err != nil {
		return s, false, errors.New("can't create kafka cache")
	}

	s.cache = kafkaCache

	topics := []string{cfg.ProducerTopic, cfg.ProducerTopicSD, cfg.ConsumerFunctionsTopic, cfg.ConsumerTaskRunnerTopic}

	if len(cfg.Brokers) == 0 || len(topics) == 0 {
		return s, false, errors.New("brokers or topics is empty")
	}

	if cfg.HealthCheckTimeout == 0 {
		return s, false, errors.New("field health_check is empty")
	}

	s.topics = topics

	metricRegistry := gometrics.DefaultRegistry
	metricRegistry.UnregisterAll()

	saramaCfg := sarama.NewConfig()

	// Required configs for exactly once delivery
	saramaCfg.Producer.Idempotent = true
	saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
	saramaCfg.Net.MaxOpenRequests = 1

	saramaCfg.MetricRegistry = metricRegistry
	saramaCfg.Producer.Return.Successes = true // Producer.Return.Successes must be true to be used in a SyncProducer
	saramaCfg.Net.DialTimeout = kafkaNetTimeout
	saramaCfg.Consumer.Group.Rebalance.GroupStrategies = s.getGroupStrategy(cfg.GroupStrategy)

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

	producerFuncResult, err := msgkit.NewProducer(saramaClient, cfg.ConsumerFunctionsTopic)
	if err != nil {
		return s, true, err
	}

	s.producerFuncResult = producerFuncResult

	consumerFunctions, err := msgkit.NewConsumer(saramaClient, cfg.ConsumerGroupFunctions, cfg.ConsumerFunctionsTopic)
	if err != nil {
		return s, true, err
	}

	consumerRunner, err := msgkit.NewConsumer(saramaClient, cfg.ConsumerGroupTaskRunner, cfg.ConsumerTaskRunnerTopic)
	if err != nil {
		return s, true, err
	}

	s.consumerFunctions = consumerFunctions
	s.consumerTaskRunner = consumerRunner

	m.KafkaAvailable()

	return s, true, nil
}

func (s *Service) ProduceFuncMessage(ctx c.Context, message *RunnerOutMessage) error {
	if s == nil || s.producer == nil || s.producerFuncResult == nil {
		return errors.New("kafka service unavailable")
	}

	return s.producer.ProduceWithKey(ctx, message.TaskID.String(), message)
}

func (s *Service) ProduceFuncResultMessage(ctx c.Context, message *RunnerInMessage) error {
	ctx, span := trace.StartSpan(ctx, "produce_func_result_message")
	defer span.End()

	if s == nil || s.producer == nil || s.producerFuncResult == nil {
		return errors.New("kafka service unavailable")
	}

	return s.producerFuncResult.ProduceWithKey(ctx, message.TaskID.String(), message)
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
	if s != nil && s.producer != nil || s.producerFuncResult == nil {
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

func (s *Service) InitMessageHandler(
	handlerFunc func(c.Context, RunnerInMessage) error,
	handlerRunTask func(c.Context, RunTaskMessage) error,
) {
	if s == nil {
		return
	}

	s.FuncMessageHandler = msgkit.NewMessageHandler[RunnerInMessage](s.log, handlerFunc, "function_return")
	s.TaskRunnerMessageHandler = msgkit.NewMessageHandler[RunTaskMessage](s.log, handlerRunTask, "run_task")
}

func (s *Service) StartConsumer(ctx c.Context) {
	if s == nil || s.consumerFunctions == nil || s.consumerTaskRunner == nil || s.isConsuming {
		return
	}

	serveCtx, cancel := c.WithCancel(ctx)
	s.ctxCancel = cancel

	s.isConsuming = true
	s.stoppedByPing = false

	go func() {
		err := s.consumerFunctions.Serve(serveCtx, s.FuncMessageHandler)
		if err != nil {
			s.log.Error(err)
		}

		s.isConsuming = false
	}()

	go func() {
		err := s.consumerTaskRunner.Serve(serveCtx, s.TaskRunnerMessageHandler)
		if err != nil {
			s.log.Error(err)
		}

		s.isConsuming = false
	}()
}

func (s *Service) StopConsumer() {
	s.ctxCancel()

	s.stoppedByPing = true
}

// nolint:gocognit //its ok here
func (s *Service) StartCheckHealth() {
	for {
		<-time.After(time.Duration(s.serviceConfig.HealthCheckTimeout) * time.Second)

		s.checkHealth()
	}
}

//nolint:nestif,gocognit //так нужно
func (s *Service) checkHealth() {
	metricRegistry := gometrics.DefaultRegistry
	metricRegistry.UnregisterAll()

	saramaCfg := sarama.NewConfig()
	saramaCfg.MetricRegistry = metricRegistry
	saramaCfg.Producer.Return.Successes = true // Producer.Return.Successes must be true to be used in a SyncProducer
	saramaCfg.Net.DialTimeout = kafkaNetTimeout

	admin, err := sarama.NewClusterAdmin(s.brokers, saramaCfg)
	if err != nil || (!s.isConsuming && !s.stoppedByPing) || s.producer == nil || s.producerFuncResult == nil {
		if err == nil {
			if s.producer == nil || s.producerFuncResult == nil {
				err = errors.New("producer is nil")
			}

			if !s.isConsuming && !s.stoppedByPing {
				err = errors.New("currently is not consuming")
			}
		}

		s.log.WithError(err).Error("couldn't connect to kafka! Trying to reconnect")
		s.metrics.KafkaUnavailable()

		msgFunc := s.FuncMessageHandler
		msgRun := s.TaskRunnerMessageHandler

		newService, _, reconnectErr := NewService(s.log, s.serviceConfig, s.metrics)
		*s = *newService
		s.FuncMessageHandler = msgFunc
		s.TaskRunnerMessageHandler = msgRun

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

func (s *Service) getGroupStrategy(assignor string) []sarama.BalanceStrategy {
	switch assignor {
	case "range":
		return []sarama.BalanceStrategy{sarama.BalanceStrategyRange}
	case "round-robin":
		return []sarama.BalanceStrategy{sarama.BalanceStrategyRoundRobin}
	case "sticky":
		return []sarama.BalanceStrategy{sarama.BalanceStrategySticky}
	default:
		s.log.Info("invalid kafka consumer group strategy in config. set default")

		return []sarama.BalanceStrategy{sarama.BalanceStrategySticky}
	}
}

// getCacheDBIdx берет номер реплики пода из его идентификатора вида hostname-{число}.
func (s *Service) getCacheDBIdx(cfg *Config) int {
	hostname := os.Getenv(cfg.PodIdxEnvKey)

	splittedStr := strings.Split(hostname, "-")

	dbIdx, err := strconv.Atoi(splittedStr[len(splittedStr)-1])
	if err != nil {
		s.log.WithError(err).Error("invalid pod index value:", splittedStr)

		return cfg.Cache.DB
	}

	return dbIdx
}
