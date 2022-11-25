package pipeline

import (
	"context"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type Status string

var (
	StatusIdle      Status = "idle"
	StatusReady     Status = "ready"
	StatusRunning   Status = "running"
	StatusFinished  Status = "finished"
	StatusNoSuccess Status = "no_success"
	StatusCancel    Status = "cancel"
)

type Runner interface {
	GetState() interface{}
	Next(runCtx *store.VariableStore) ([]string, bool)
	Update(ctx context.Context) (interface{}, error)
	GetTaskHumanStatus() TaskHumanStatus
	GetStatus() Status
	UpdateManual() bool
	Members() map[string]struct{}
	CheckSLA() (bool, bool, time.Time)
}
