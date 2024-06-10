package people

import (
	c "context"

	"github.com/hashicorp/go-retryablehttp"
)

type Service interface {
	Setter

	GetUserEmail(ctx c.Context, username string) (string, error)
	GetUser(ctx c.Context, search string, onlyEnabled bool) (SSOUser, error)
	GetUsers(ctx c.Context, search string, limit *int, filter []string, enabled bool) ([]SSOUser, error)
	Ping(ctx c.Context) error
}

type Setter interface {
	SetCli(cli *retryablehttp.Client)
}
