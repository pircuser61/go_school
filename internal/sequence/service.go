package sequence

import (
	c "context"
	"fmt"
	"sync"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

type service struct {
	PrefetchSize         int
	prefetchMinQueueSize int
	q                    utils.FifoQueue
	log                  logger.Logger
	m                    metrics.Metrics
	mx                   sync.Mutex
}

func NewService(cfg Config, log logger.Logger, m metrics.Metrics) (Service, error) {
	q := utils.CreateQueue(cfg.WorkNumberPrefetchSize + cfg.PrefetchMinQueueSize)

	return &service{
		PrefetchSize:         cfg.WorkNumberPrefetchSize,
		prefetchMinQueueSize: cfg.PrefetchMinQueueSize,
		q:                    q,
		log:                  log,
		m:                    m,
	}, nil
}

func (s *service) Lock() {
	// костыль из-за архитектуры приложения
	s.mx.Lock()
}

func (s *service) Unlock() {
	s.mx.Unlock()
}

func (s *service) GetPrefetchSize() int {
	return s.PrefetchSize
}

func (s *service) GetWorkNumberFromQueue(ctx c.Context) (workNumber string, ok, needPrefetch bool) {
	//nolint:ineffassign //it's ok
	ctx, span := trace.StartSpan(ctx, "sequence.get_work_number")
	defer span.End()

	if s.q.Length() <= s.prefetchMinQueueSize {
		needPrefetch = true
	}

	workNumberInt, ok := s.q.Pop()

	return fmt.Sprintf("J%014d", workNumberInt), ok, needPrefetch
}

func (s *service) AddWorkNumbersToQueue(workNumbers []int) {
	s.q.BulkPush(workNumbers)
}

func (s *service) AddWorkNumberToQueue(workNumber int) {
	s.q.Push(workNumber)
}
