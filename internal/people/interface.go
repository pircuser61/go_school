package people

import (
	"context"
)

type ServiceInterface interface {
	PathBuilder(mainpath, subpath string) (string, error)
	GetUserEmail(ctx context.Context, username string) (string, error)
	GettingUser(ctx context.Context, username string) (SSOUser, error)
	GettingUsers(ctx context.Context, username string, limit *int, filter []string) ([]SSOUser, error)
	GetUser(ctx context.Context, search string, onlyEnabled bool) ([]SSOUser, error)
	GetUsers(ctx context.Context, search string, limit int, filter []string) ([]SSOUser, error)
}
