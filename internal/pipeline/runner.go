package pipeline

import (
	"context"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

type Runner interface {
	Run(ctx context.Context, runCtx *store.VariableStore) error
	Next() string
}
