package servicedesc

import (
	c "context"
	"errors"
	"io"
	"net/http"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type Service struct {
	sdURL string

	cli *http.Client
}

func NewService(cfg Config) (*Service, error) {
	newCli := &http.Client{}

	s := &Service{
		cli:   newCli,
		sdURL: cfg.ServicedeskURL,
	}

	return s, nil
}

func makeRequest(ctx c.Context, method, url string, body io.Reader) (req *http.Request, err error) {
	req, err = http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	token := ctx.Value(script.AuthorizationHeader{})

	if token == nil {
		return nil, errors.New("auth token is nil")
	}

	stringToken, ok := token.(string)
	if !ok {
		return nil, errors.New("can`t cast auth token to string")
	}

	req.Header.Add(authorizationHeader, stringToken)

	return req, nil
}
