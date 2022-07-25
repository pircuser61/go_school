package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	PipelineRunFailed     = "pipeline run failed"
	GetPipelineTaskFailed = "can't get pipeline tasks"
	EmptyResponse         = "empty response"
	CreateRequstError     = "can't create http request"
	HTTPClientError       = "can't do http request"
	MarshalJSONError      = "can't marshal json"
	UnmarshalReadError    = "can't read response body"
	UnmarshalJSONError    = "can't umarshal json"
)

type PipelinerConfig struct {
	Host URL    `json:"host"`
	User string `json:"user"`
}

type RunResponseHTTP struct {
	StatusCode int         `json:"status_code"`
	Data       RunResponse `json:"data,omitempty"`
}

type RunResponse struct {
	PipelineID uuid.UUID   `json:"pipeline_id"`
	TaskID     uuid.UUID   `json:"task_id"`
	Status     string      `json:"status"`
	Output     interface{} `json:"output"`
	Errors     []string    `json:"errors"`
}

type EriusTaskHTTP struct {
	StatusCode int       `json:"status_code"`
	Data       EriusTask `json:"data,omitempty"`
}

type EriusTask struct {
	ID          uuid.UUID              `json:"id"`
	VersionID   uuid.UUID              `json:"version_id"`
	StartedAt   time.Time              `json:"started_at"`
	Status      string                 `json:"status"`
	Author      string                 `json:"author"`
	IsDebugMode bool                   `json:"debug"`
	Parameters  map[string]interface{} `json:"parameters"`
	Steps       TaskSteps              `json:"steps"`
}

type TaskSteps []*Step

type Step struct {
	Time        time.Time              `json:"time"`
	Name        string                 `json:"name"`
	Storage     map[string]interface{} `json:"storage"`
	Errors      []string               `json:"errors"`
	Steps       []string               `json:"steps"`
	BreakPoints []string               `json:"-"`
	HasError    bool                   `json:"has_error"`
}

type URL struct {
	*url.URL
}

func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	stringURL := ""

	err := unmarshal(&stringURL)
	if err != nil {
		return err
	}

	u.URL, err = url.Parse(stringURL)

	return err
}

type PipelinerClient struct {
	client *http.Client
	user   string
	host   url.URL
}

func NewPipelinerClient(pc PipelinerConfig, httpClient *http.Client) *PipelinerClient {
	return &PipelinerClient{
		client: httpClient,
		user:   pc.User,
		host:   *pc.Host.URL,
	}
}

func (pc *PipelinerClient) RunPipeline(ctx context.Context, pid fmt.Stringer, data interface{}, sync bool) (*RunResponse, error) {
	params := url.Values{}
	params.Add("with_stop", strconv.FormatBool(sync))

	urlCopy := pc.host
	urlCopy.Path = path.Join(pc.host.Path, "api/pipeliner/v1/run", pid.String())
	urlCopy.RawQuery = params.Encode()

	b, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(err, MarshalJSONError)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlCopy.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.Wrap(err, CreateRequstError)
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-ERIUS-USER", pc.user)

	result, statusCode, err := doRequest[RunResponseHTTP](ctx, pc.client, req)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, errors.New(PipelineRunFailed)
	}

	return &result.Data, nil
}

func (pc *PipelinerClient) GetTasks(ctx context.Context, taskID fmt.Stringer) (*EriusTask, error) {
	urlCopy := pc.host
	urlCopy.Path = path.Join(pc.host.Path, "api/pipeliner/v1/tasks", taskID.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlCopy.String(), http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, CreateRequstError)
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-ERIUS-USER", pc.user)

	result, statusCode, err := doRequest[EriusTaskHTTP](ctx, pc.client, req)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, errors.New(GetPipelineTaskFailed)
	}

	return &result.Data, nil
}

// doRequest - рутинная функа, делает запрос и анмаршаллит данные
func doRequest[V any](ctx context.Context, client *http.Client, req *http.Request) (v *V, i int, err error) {
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, errors.Wrap(err, HTTPClientError)
	}

	res, err := unmarshalBody[V](resp)
	if err != nil {
		return nil, 0, err
	}

	return res, resp.StatusCode, nil
}

func unmarshalBody[V any](res *http.Response) (*V, error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, UnmarshalReadError)
	}

	defer res.Body.Close()

	if len(body) == 0 {
		return nil, errors.New(EmptyResponse)
	}

	var structToFill V
	err = json.Unmarshal(body, &structToFill)
	if err != nil {
		return nil, errors.Wrap(err, UnmarshalJSONError)
	}

	return &structToFill, nil
}
