package people

import (
	c "context"
)

type Service interface {
	GetUserEmail(ctx c.Context, username string) (string, error)
	GetUser(ctx c.Context, search string, onlyEnabled bool) (SSOUser, error)
	GetUsers(ctx c.Context, search string, limit *int, filter []string) ([]SSOUser, error)
	Ping(ctx c.Context) error
}
