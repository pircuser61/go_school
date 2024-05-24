package people

import (
	c "context"

	"github.com/hashicorp/go-retryablehttp"
)

type Service interface {
	Setter

	PathBuilder(mainPath, subPath string) (string, error)
	GetUserEmail(ctx c.Context, username string) (string, error)
	GetUser(ctx c.Context, search string) (SSOUser, error)
	GetUsers(ctx c.Context, search string, limit *int, filter []string) ([]SSOUser, error)
	Ping() error
}

type Setter interface {
	SetCli(cli *retryablehttp.Client)
}
