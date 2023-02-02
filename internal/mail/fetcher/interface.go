package fetcher

import c "context"

type Service interface {
	FetchEmails(ctx c.Context) ([]ParsedEmail, error)
	CloseIMAP(ctx c.Context)
}
