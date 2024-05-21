package metrics

import (
	"net/http"
	"time"
)

type PushConfig struct {
	URL string `yaml:"url"`
	Job string `yaml:"job"`
}

type PrometheusConfig struct {
	Stand string     `yaml:"stand"`
	Push  PushConfig `yaml:"push"`
}

type RequestInfo struct {
	Method     string
	Path       string
	PipelineID string
	VersionID  string
	ClientID   string
	WorkNumber string
	Status     int
	Duration   time.Duration
}

type ExternalRequestInfo struct {
	ExternalSystem string
	Method         string
	URL            string
	TraceID        string
	ResponseCode   int
	Duration       time.Duration
}

func NewExternalRequestInfo(name string) *ExternalRequestInfo {
	return &ExternalRequestInfo{
		ExternalSystem: name,
	}
}

func NewPostRequestInfo(path string) *RequestInfo {
	return &RequestInfo{
		Method: http.MethodPost,
		Status: http.StatusOK,
		Path:   path,
	}
}

func NewGetRequestInfo(path string) *RequestInfo {
	return &RequestInfo{
		Method: http.MethodGet,
		Status: http.StatusOK,
		Path:   path,
	}
}
