package handlers

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/libs/logger"
)

type APIEnv struct {
	DB            db.Database
	Logger        logger.Logger
	ScriptManager string
	FaaS          string
}
