package humantasks

import (
	c "context"

	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

type Service interface {
	Setter

	GetDelegations(ctx c.Context, req *d.GetDelegationsRequest) (ds Delegations, err error)
	GetDelegationsFromLogin(ctx c.Context, login string) (ds Delegations, err error)
	GetDelegationsToLogin(ctx c.Context, login string) (ds Delegations, err error)
	GetDelegationsToLogins(ctx c.Context, logins []string) (ds Delegations, err error)
	GetDelegationsByLogins(ctx c.Context, logins []string) (ds Delegations, err error)
	Ping(ctx c.Context) error
}

type Setter interface {
	SetCli(cli d.DelegationServiceClient)
}
