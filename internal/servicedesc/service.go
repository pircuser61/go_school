package servicedesc

import (
	c "context"
	"github.com/pkg/errors"
	"io"
	"net/http"
)

const AuthorizationHeader = "Authorization"

type Service struct {
	chainsmithURL string

	cli *http.Client
}

func NewService(c Config) (*Service, error) {
	newCli := &http.Client{}

	s := &Service{
		cli:           newCli,
		chainsmithURL: c.ChainsmithURL,
	}

	return s, nil
}


func makeRequest(ctx c.Context, method, url string, body io.Reader) (req *http.Request, err error) {
	req, err = http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	token := ctx.Value(AuthorizationHeader)

	if token == nil {
		return nil, errors.New("auth token is nil")
	}

	stringToken, ok := token.(string)
	if !ok {
		return nil, errors.New("can`t cast auth token to string")
	}

	req.Header.Add(AuthorizationHeader, stringToken)

	return req, nil
}
