package sso

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

func getTokenInCookie(req *http.Request, name string) (string, error) {
	c, err := req.Cookie(name)
	if err != nil {
		return "", errors.New("can't find user session")
	}
	return c.Value, nil
}

func getTokenInBearer(req *http.Request) (string, error) {
	token := req.Header.Get(authHeader)
	if token == "" {
		return "", errors.New("can't find user session")
	}

	items := strings.Split(token, " ")
	if len(items) != 2 {
		return "", errors.New("got bad user session")
	}

	if items[0] != authType {
		return "", errors.New("can't find user session")
	}
	return items[1], nil
}

func (s *Service) GetAccessToken(req *http.Request) (string, error) {
	token, err := getTokenInBearer(req)
	if err != nil {
		token, err = getTokenInCookie(req, s.accessTokenCookieName)
		if err != nil {
			return "", err
		}
		return token, nil
	}
	return token, nil
}
