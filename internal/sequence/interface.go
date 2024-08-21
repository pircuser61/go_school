package sequence

import c "context"

type Service interface {
	GetWorkNumber(ctx c.Context) (workNumber string, err error)
	Ping(ctx c.Context) error
}
