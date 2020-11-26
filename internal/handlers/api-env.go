package handlers

import (
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
)

type APIEnv struct {
	DB              db.Database
	Logger          logger.Logger
	ScriptManager   string
	Remedy          string
	FaaS            string
	AuthClient      *auth.Client
	SchedulerClient scheduler.Client
	HTTPClient      *http.Client
}
