package fetcher

import c "context"

type Service interface {
	FetchEmails(ctx c.Context) error
	CloseIMAP(ctx c.Context)
}
