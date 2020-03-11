package app

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/libs/logger"
)

type Pipeliner struct {
	DBConnection db.DBConn
	Logger       logger.Logger
}
