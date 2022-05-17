package handlers

import (
	"net/http"

	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"

	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
)

type APIEnv struct {
	DB                   db.Database
	ScriptManager        string
	Remedy               string
	FaaS                 string
	SchedulerClient      scheduler.Client
	NetworkMonitorClient netmon.Client
	HTTPClient           *http.Client
	Statistic            *statistic.Statistic
}
