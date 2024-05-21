package functions

import c "context"

type Service interface {
	GetFunctionVersion(ctx c.Context, functionID, versionID string) (res Function, err error)
	GetFunction(ctx c.Context, id string) (result Function, err error)
}
