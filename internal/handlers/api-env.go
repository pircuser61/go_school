package handlerst a

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/libs/logger"
)

type ApiEnv struct {
	DBConnection  *dbconn.PGConnection
	Logger        logger.Logger
	ScriptManager string
}
