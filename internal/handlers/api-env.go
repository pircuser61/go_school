package handlers

import (
	"net/http"

	"gitlab.services.mts.ru/erius/pipeliner/statistic"

	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
)

type APIEnv struct {
	DB                   db.Database
	ScriptManager        string
	Remedy               string
	FaaS                 string
	AuthClient           *auth.Client
	SchedulerClient      scheduler.Client
	NetworkMonitorClient netmon.Client
	HTTPClient           *http.Client
	Statistic            *statistic.Statistic
}
