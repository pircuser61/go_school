package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type Runner interface {
	GetType() string
	GetState() interface{}
	DebugRun(ctx context.Context, runCtx *store.VariableStore) error
	Run(ctx context.Context, runCtx *store.VariableStore) error
	Next(runCtx *store.VariableStore) (string, bool)
	NextSteps() []string
	IsScenario() bool
	Inputs() map[string]string
	Outputs() map[string]string
	Update(ctx context.Context, data *script.BlockUpdateData) (interface{}, error)
}
