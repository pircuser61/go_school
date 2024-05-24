package server

import (
	c "context"
	"sync"
	"time"
)

const countServices = 11

//nolint:all // ok
func (s *service) PingServices(ctx c.Context, failedCh chan bool) {
	var (
		kafkaStopped bool
		failedCount  int
		okCount      int
	)

	for {
		<-time.After(s.servicesPing.PingTimer)
		var dbErr, schedulerErr error

		var wg sync.WaitGroup
		wg.Add(countServices)

		go func() {
			defer wg.Done()

			dbErr = s.apiEnv.DB.Ping(ctx)
			if dbErr == nil {
				s.metrics.DBAvailable()
			} else {
				s.metrics.DBUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			schedulerErr = s.apiEnv.Scheduler.Ping(ctx)
			if schedulerErr == nil {
				s.metrics.SchedulerAvailable()
			} else {
				s.metrics.SchedulerUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			fileRegistryErr := s.apiEnv.FileRegistry.Ping(ctx)
			if fileRegistryErr == nil {
				s.metrics.FileRegistryAvailable()
			} else {
				s.metrics.FileRegistryUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			humanTasksErr := s.apiEnv.HumanTasks.Ping(ctx)
			if humanTasksErr == nil {
				s.metrics.HumanTasksAvailable()
			} else {
				s.metrics.HumanTasksUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			functionStoreErr := s.apiEnv.FunctionStore.Ping(ctx)
			if functionStoreErr == nil {
				s.metrics.FunctionStoreAvailable()
			} else {
				s.metrics.FunctionStoreUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			serviceDescErr := s.apiEnv.ServiceDesc.Ping()
			if serviceDescErr == nil {
				s.metrics.ServiceDescAvailable()
			} else {
				s.metrics.ServiceDescUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			peopleErr := s.apiEnv.People.Ping(ctx)
			if peopleErr == nil {
				s.metrics.PeopleAvailable()
			} else {
				s.metrics.PeopleStoreUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			mailErr := s.apiEnv.Mail.Ping()
			if mailErr == nil {
				s.metrics.MailAvailable()
			} else {
				s.metrics.MailUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			integrationsErr := s.apiEnv.Integrations.Ping()
			if integrationsErr == nil {
				s.metrics.IntegrationsAvailable()
			} else {
				s.metrics.IntegrationsUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			hrGateErr := s.apiEnv.HrGate.Ping()
			if hrGateErr == nil {
				s.metrics.HrGateAvailable()
			} else {
				s.metrics.HrGateUnavailable()
			}
		}()

		go func() {
			defer wg.Done()

			sequenceErr := s.apiEnv.Sequence.Ping(ctx)
			if sequenceErr == nil {
				s.metrics.SequenceAvailable()
			} else {
				s.metrics.SequenceUnavailable()
			}
		}()

		wg.Wait()

		if dbErr != nil || schedulerErr != nil {
			if dbErr != nil {
				s.logger.WithError(dbErr).Error("DB not accessible")
			}

			if schedulerErr != nil {
				s.logger.WithError(schedulerErr).Error("scheduler not accessible")
			}

			if kafkaStopped {
				continue
			}

			okCount = 0

			failedCount++
			if failedCount < s.servicesPing.MaxFailedCnt {
				continue
			}

			kafkaStopped = true
			failedCh <- true

			s.logger.Error("kafka consume stop")

			continue
		}

		if !kafkaStopped {
			continue
		}

		okCount++
		if okCount < s.servicesPing.MaxOkCnt {
			continue
		}

		failedCount = 0

		kafkaStopped = false
		failedCh <- false

		s.logger.Info("kafka consume start")
	}
}
