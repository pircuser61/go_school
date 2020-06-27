package handlers

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/libs/logger"
)

type APIEnv struct {
	DBConnection  *dbconn.PGConnection
	Logger        logger.Logger
	ScriptManager string
	FaaS          string
}
