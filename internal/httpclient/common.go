package httpclient

import "net/http"

type AuthFunc func() string

// https://stackoverflow.com/questions/43447405/change-http-client-transport
type Transport struct {
	rt       http.RoundTripper
	authFunc AuthFunc
}

func NewTransport(rt http.RoundTripper, authFunc AuthFunc) *Transport {
	return &Transport{
		rt,
		authFunc,
	}
}

func (tc *Transport) transport() http.RoundTripper {
	if tc.rt != nil {
		return tc.rt
	}

	return http.DefaultTransport
}

// RoundTrip - проставляет хэдеры, если не были установлены
func (tc *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Content-Type") == "" {
		req.Header.Add("Content-Type", "application/json")
	}

	if req.Header.Get("Accept") == "" {
		req.Header.Add("Accept", "application/json")
	}

	if req.Header.Get("Authorization") == "" && tc.authFunc != nil {
		req.Header.Set("Authorization", tc.authFunc())
	}

	return tc.transport().RoundTrip(req)
}
