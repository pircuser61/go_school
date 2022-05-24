package httpclient

import (
	"net"
	"net/http"
	"net/url"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
)

func NewProxyFunc(proxyAddr string) func(*http.Request) (*url.URL, error) {
	var proxyFunc func(*http.Request) (*url.URL, error)

	proxyURL, proxyErr := url.Parse(proxyAddr)
	if proxyErr == nil && proxyAddr != "" {
		proxyFunc = http.ProxyURL(proxyURL)
	}

	return proxyFunc
}

func HTTPClient(config *configs.HTTPClient) *http.Client {
	return &http.Client{
		Transport: &ochttp.Transport{
			Base: &http.Transport{
				Proxy: NewProxyFunc(config.ProxyURL.String()),
				DialContext: (&net.Dialer{
					Timeout:   config.Timeout.Duration,
					KeepAlive: config.KeepAlive.Duration,
				}).DialContext,
				MaxIdleConns:          config.MaxIdleConns,
				IdleConnTimeout:       config.IdleConnTimeout.Duration,
				TLSHandshakeTimeout:   config.TLSHandshakeTimeout.Duration,
				ExpectContinueTimeout: config.ExpectContinueTimeout.Duration,
			},
		},
	}
}
