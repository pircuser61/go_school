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

	"github.com/pkg/errors"
)

const (
	PipelineRunFailed  = "pipeline run failed"
	EmptyResponse      = "empty response"
	CreateRequstError  = "can't create http request"
	HTTPClientError    = "can't do http request"
	MarshalJSONError   = "can't marshal json"
	UnmarshalReadError = "can't read response body"
	UnmarshalJSONError = "can't umarshal json"
)

type PipelinerConfig struct {
	Host URL    `json:"host"`
	User string `json:"user"`
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

func (pc *PipelinerClient) RunPipelineSync(ctx context.Context, pid fmt.Stringer, data interface{}) (map[string]interface{}, error) {
	params := url.Values{}
	params.Add("with_stop", strconv.FormatBool(true))

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

	resp, err := pc.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, HTTPClientError)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(PipelineRunFailed)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, UnmarshalReadError)
	}

	if len(body) == 0 {
		return nil, errors.New(EmptyResponse)
	}

	result := make(map[string]interface{})

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, errors.Wrap(err, UnmarshalJSONError)
	}

	return result, nil
}
