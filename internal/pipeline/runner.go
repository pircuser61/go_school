package pipeline

import (
	"context"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type Status string

const (
	ActionTypePrimary   = "primary"
	ActionTypeSecondary = "secondary"
	ActionTypeOther     = "other"
	ActionTypeCustom    = "custom"
)

type MemberAction struct {
	Id   string
	Type string
}
type Member struct {
	Login      string
	IsFinished bool
	Actions    []MemberAction
}

type Deadline struct {
	Deadline time.Time
	Action   entity.TaskUpdateAction
}

var (
	StatusIdle      Status = "idle"
	StatusReady     Status = "ready"
	StatusRunning   Status = "running"
	StatusFinished  Status = "finished"
	StatusNoSuccess Status = "no_success"
	StatusError     Status = "error"
)

type Runner interface {
	GetState() interface{}
	Next(runCtx *store.VariableStore) ([]string, bool)
	Update(ctx context.Context) (interface{}, error)
	GetTaskHumanStatus() TaskHumanStatus
	GetStatus() Status
	UpdateManual() bool
	Members() []Member
	Deadlines(ctx context.Context) ([]Deadline, error)
}
