package hrgate

import (
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

type httpRequestDoer struct {
	*retryablehttp.Client
}

func (h httpRequestDoer) Do(req *http.Request) (*http.Response, error) {
	wrappedRequest, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	return h.Client.Do(wrappedRequest)
}
