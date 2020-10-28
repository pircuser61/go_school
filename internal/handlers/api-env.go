package handlers

import (
	"net/http"

	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/libs/logger"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
)

type APIEnv struct {
	DB            db.Database
	Logger        logger.Logger
	ScriptManager string
	Remedy        string
	FaaS          string
	AuthClient    *auth.Client
	HTTPClient    *http.Client
}
