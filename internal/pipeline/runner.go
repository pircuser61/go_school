package pipeline

import (
	"context"

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
	GetType() string
	GetState() interface{}
	DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) error
	Next(runCtx *store.VariableStore) ([]string, bool)
	Skipped(runCtx *store.VariableStore) []string
	IsScenario() bool
	Inputs() map[string]string
	Outputs() map[string]string
	Update(ctx context.Context) (interface{}, error)
	GetTaskHumanStatus() TaskHumanStatus
	GetStatus() Status
	UpdateManual() bool
	Members() map[string]struct{}
	CheckSLA() bool
}
