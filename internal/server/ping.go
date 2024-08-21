package server

import (
	c "context"
	"sync"
	"time"
)

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

		s.pings = map[string]func(){
			"db": func() {
				defer wg.Done()

				dbErr = s.apiEnv.DB.Ping(ctx)
				if dbErr == nil {
					s.metrics.DBAvailable()
				} else {
					s.metrics.DBUnavailable()
				}
			},
			"scheduler": func() {
				defer wg.Done()

				schedulerErr = s.apiEnv.Scheduler.Ping(ctx)
				if schedulerErr == nil {
					s.metrics.SchedulerAvailable()
				} else {
					s.metrics.SchedulerUnavailable()
				}
			},
			"fileRegistry": func() {
				defer wg.Done()

				fileRegistryErr := s.apiEnv.FileRegistry.Ping(ctx)
				if fileRegistryErr == nil {
					s.metrics.FileRegistryAvailable()
				} else {
					s.metrics.FileRegistryUnavailable()
				}
			},
			"humanTasks": func() {
				defer wg.Done()

				humanTasksErr := s.apiEnv.HumanTasks.Ping(ctx)
				if humanTasksErr == nil {
					s.metrics.HumanTasksAvailable()
				} else {
					s.metrics.HumanTasksUnavailable()
				}
			},
			"functionStore": func() {
				defer wg.Done()

				functionStoreErr := s.apiEnv.FunctionStore.Ping(ctx)
				if functionStoreErr == nil {
					s.metrics.FunctionStoreAvailable()
				} else {
					s.metrics.FunctionStoreUnavailable()
				}
			},
			"serviceDesc": func() {
				defer wg.Done()

				serviceDescErr := s.apiEnv.ServiceDesc.Ping(ctx)
				if serviceDescErr == nil {
					s.metrics.ServiceDescAvailable()
				} else {
					s.metrics.ServiceDescUnavailable()
				}
			},
			"people": func() {
				defer wg.Done()

				peopleErr := s.apiEnv.People.Ping(ctx)
				if peopleErr == nil {
					s.metrics.PeopleAvailable()
				} else {
					s.metrics.PeopleStoreUnavailable()
				}
			},
			"mail": func() {
				defer wg.Done()

				mailErr := s.apiEnv.Mail.Ping(ctx)
				if mailErr == nil {
					s.metrics.MailAvailable()
				} else {
					s.metrics.MailUnavailable()
				}
			},
			"integrations": func() {
				defer wg.Done()

				integrationsErr := s.apiEnv.Integrations.Ping(ctx)
				if integrationsErr == nil {
					s.metrics.IntegrationsAvailable()
				} else {
					s.metrics.IntegrationsUnavailable()
				}
			},
			"hrGate": func() {
				defer wg.Done()

				hrGateErr := s.apiEnv.HrGate.Ping(ctx)
				if hrGateErr == nil {
					s.metrics.HrGateAvailable()
				} else {
					s.metrics.HrGateUnavailable()
				}
			},
		}

		wg.Add(len(s.pings))

		for serviceName := range s.pings {
			go s.pings[serviceName]()
		}

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
