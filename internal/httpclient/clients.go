package httpclient

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type Config struct {
	Timeout               time.Duration `yaml:"timeout"`
	KeepAlive             time.Duration `yaml:"keep_alive"`
	MaxIdleConns          int           `yaml:"max_idle_conns"`
	IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
	TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
	ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"`
	ProxyURL              script.URL    `yaml:"proxy_url"`

	MaxRetries uint          `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
}

func NewProxyFunc(proxyAddr string) func(*http.Request) (*url.URL, error) {
	var proxyFunc func(*http.Request) (*url.URL, error)

	proxyURL, proxyErr := url.Parse(proxyAddr)
	if proxyErr == nil && proxyAddr != "" {
		proxyFunc = http.ProxyURL(proxyURL)
	}

	return proxyFunc
}

func HTTPClient(config *Config) *http.Client {
	return &http.Client{
		Transport: &ochttp.Transport{
			Base: &http.Transport{
				Proxy: NewProxyFunc(config.ProxyURL.String()),
				DialContext: (&net.Dialer{
					Timeout:   config.Timeout,
					KeepAlive: config.KeepAlive,
				}).DialContext,
				MaxIdleConns:          config.MaxIdleConns,
				IdleConnTimeout:       config.IdleConnTimeout,
				TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
				ExpectContinueTimeout: config.ExpectContinueTimeout,
			},
		},
	}
}

func HTTPClientWithRetries(client *http.Client, log logger.Logger, maxRetries uint, retryDelay time.Duration) *retryablehttp.Client {
	return &retryablehttp.Client{
		HTTPClient: client,
		Logger:     log,
		RetryMax:   int(maxRetries),
		CheckRetry: retryablehttp.DefaultRetryPolicy,
		Backoff: func(_, _ time.Duration, _ int, _ *http.Response) time.Duration {
			return retryDelay
		},
	}
}
