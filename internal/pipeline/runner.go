package pipeline

import "context"

type Runner interface {
	Run(ctx context.Context, runCtx *VariableStore) error
	Next() string
}
