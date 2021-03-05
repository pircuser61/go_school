package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/google/uuid"
)

var PipelineRunFailed = errors.New("pipeline run failed")

type PipelinerConfig struct {
	Host    string        `json:"host"`
	User    string        `json:"user"`
	Timeout time.Duration `json:"timeout"`
}

type PipelinerClient struct {
	client *http.Client
	user   string
	host   string
}

func NewPipelinerClient(pc PipelinerConfig) (*PipelinerClient, error) {
	cli := &http.Client{
		Timeout: pc.Timeout,
	}

	return &PipelinerClient{
		client: cli,
		user:   pc.User,
		host:   pc.Host,
	}, nil
}

func (pc *PipelinerClient) RunPipelineSync(ctx context.Context, pid uuid.UUID, data interface{}) (map[string]interface{}, error) {
	u, err := url.Parse(path.Join(pc.host, "api/pipeliner/v1/run", pid.String()+"?with_stop=true"))
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("X-ERIUS-USER", pc.user)

	resp, err := pc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, PipelineRunFailed
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, http.ErrContentLength
	}

	result := make(map[string]interface{})

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
