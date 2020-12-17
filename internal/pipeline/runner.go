package pipeline

import (
	"context"

	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

type Runner interface {
	DebugRun(ctx context.Context, runCtx *store.VariableStore) error
	Run(ctx context.Context, runCtx *store.VariableStore) error
	Next(runCtx *store.VariableStore) string
	IsScenario() bool
	Inputs() map[string]string
	Outputs() map[string]string
}
