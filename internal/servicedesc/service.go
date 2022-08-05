package servicedesc

import (
	"net/http"
)

type Service struct {
	chainsmithURL string

	cli *http.Client
}

func NewService(c Config) (*Service, error) {
	newCli := &http.Client{}

	s := &Service{
		cli: newCli,
	}

	return s, nil
}
