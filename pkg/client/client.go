package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

type RunResponse struct {
	PipelineID uuid.UUID   `json:"pipeline_id"`
	TaskID     uuid.UUID   `json:"task_id"`
	Status     string      `json:"status"`
	Output     interface{} `json:"output"`
	Errors     []string    `json:"errors"`
}

type EriusTasks struct {
	Tasks []EriusTask `json:"tasks"`
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
	host   *url.URL
}

func NewPipelinerClient(pc PipelinerConfig, httpClient *http.Client) *PipelinerClient {
	return &PipelinerClient{
		client: httpClient,
		user:   pc.User,
		host:   pc.Host.URL,
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

	result := RunResponse{}

	statusCode, err := doRequest(ctx, pc.client, req, &result)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, errors.New(PipelineRunFailed)
	}

	return &result, nil
}

func (pc *PipelinerClient) GetTasks(ctx context.Context, taskID fmt.Stringer) (*EriusTasks, error) {
	urlCopy := pc.host
	urlCopy.Path = path.Join(pc.host.Path, "api/pipeliner/v1/tasks", taskID.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlCopy.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, CreateRequstError)
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-ERIUS-USER", pc.user)

	result := EriusTasks{}

	statusCode, err := doRequest(ctx, pc.client, req, &result)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, errors.New(GetPipelineTaskFailed)
	}

	return &result, nil
}

// doRequest - рутинная функа, делает запрос и анмаршаллит данные
func doRequest(ctx context.Context, client *http.Client, req *http.Request, responseStruct interface{}) (int, error) {
	req = req.WithContext(ctx)

	res, err := client.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, HTTPClientError)
	}

	if responseStruct != nil {
		err := unmarshalBody(res, responseStruct)
		if err != nil {
			return 0, err
		}
	}

	return res.StatusCode, nil
}

func unmarshalBody(res *http.Response, structToFill interface{}) error {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, UnmarshalReadError)
	}

	defer res.Body.Close()

	if len(body) == 0 {
		return errors.New(EmptyResponse)
	}

	err = json.Unmarshal(body, structToFill)
	if err != nil {
		return errors.Wrap(err, UnmarshalJSONError)
	}

	return nil
}
