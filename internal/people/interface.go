package people

import (
	"context"
)

type Service interface {
	PathBuilder(mainpath, subpath string) (string, error)
	GetUserEmail(ctx context.Context, username string) (string, error)
	GetUser(ctx context.Context, search string) (SSOUser, error)
	GetUsers(ctx context.Context, search string, limit *int, filter []string) ([]SSOUser, error)
}
