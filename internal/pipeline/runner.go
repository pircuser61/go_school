package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type Runner interface {
	DebugRun(ctx context.Context, runCtx *store.VariableStore) error
	Run(ctx context.Context, runCtx *store.VariableStore) error
	Next(runCtx *store.VariableStore) (string, bool)
	NextSteps() []string
	IsScenario() bool
	Inputs() map[string]string
	Outputs() map[string]string
}
