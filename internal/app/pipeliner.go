package app

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/libs/logger"
)

type Pipeliner struct {
	DBConnection *dbconn.PGConnection
	Logger       logger.Logger
}
