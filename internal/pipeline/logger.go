package pipeline

import (
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/libs/logger"
	"io"
)

type Logger struct {
	TextLogger logger.Logger
	DB         *dbconn.PGConnection
}

func NewLogger(log logger.Logger, db *dbconn.PGConnection) *Logger {
	return &Logger{
		TextLogger: log,
		DB:         db,
	}
}

func (l *Logger) Debug(args ...interface{}) {
	l.TextLogger.Debug(args)
}
func (l *Logger) Info(args ...interface{}) {
	l.TextLogger.Info(args)
}
func (l *Logger) Warning(args ...interface{}) {}
func (l *Logger) Error(args ...interface{})   {}

func (l *Logger) Tracef(format string, args ...interface{})   {}
func (l *Logger) Debugf(format string, args ...interface{})   {}
func (l *Logger) Infof(format string, args ...interface{})    {}
func (l *Logger) Warningf(format string, args ...interface{}) {}
func (l *Logger) Errorf(format string, args ...interface{})   {}

func (l *Logger) Print(...interface{})          {}
func (l *Logger) Printf(string, ...interface{}) {}
func (l *Logger) Println(...interface{})        {}

func (l *Logger) Fatal(...interface{})          {}
func (l *Logger) Fatalf(string, ...interface{}) {}
func (l *Logger) Fatalln(...interface{})        {}

func (l *Logger) Panic(...interface{})          {}
func (l *Logger) Panicf(string, ...interface{}) {}
func (l *Logger) Panicln(...interface{})        {}

func (l *Logger) Traceln(...interface{})   {}
func (l *Logger) Debugln(...interface{})   {}
func (l *Logger) Infoln(...interface{})    {}
func (l *Logger) Warningln(...interface{}) {}
func (l *Logger) Errorln(...interface{})   {}

func (l *Logger) Log(level logger.Level, args ...interface{})                 {}
func (l *Logger) Logf(level logger.Level, format string, args ...interface{}) {}
func (l *Logger) Logln(level logger.Level, args ...interface{})               {}

func (l *Logger) Writer() *io.PipeWriter {
	return l.TextLogger.Writer()
}

func (l *Logger) WithField(key string, value interface{}) Logger {
	return Logger{}
}
func (l *Logger) WithFields(fields logger.Fields) Logger {
	return Logger{}
}
func (l *Logger) WithError(err error) Logger {
	return Logger{}
}
