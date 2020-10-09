package handlers

import (
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/libs/logger"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
)

type APIEnv struct {
	DB            db.Database
	Logger        logger.Logger
	ScriptManager string
	FaaS          string
	AuthClient    *auth.Client
}
