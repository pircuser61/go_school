package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

type RemedySendUpdateWork struct {
	Name       string
	NextBlock  string
	Input      map[string]string
	HTTPClient http.Client
	Remedy     string
}

type RemedySendUpdateWorkModel struct {
	ExtID           string    `json:"ExtID,omitempty"`
	RequestID       string    `json:"Request_id,omitempty"`
	OperationID     string    `json:"OperationID,omitempty"`
	Status          string    `json:"Status,omitempty"`
	Priority        string    `json:"Priority,omitempty"`
	PlanStart       time.Time `json:"PlanStart,omitempty"`
	DeadLine        time.Time `json:"DeadLine,omitempty"`
	Start           time.Time `json:"Start,omitempty"`
	Finish          time.Time `json:"Finish,omitempty"`
	ExecutorGroupID string    `json:"ExecutorGroupID,omitempty"`
	ExecutorLogin   string    `json:"ExecutorLogin,omitempty"`
	Description     string    `json:"Description,omitempty"`
	MainNIOSSID     string    `json:"Main_NIOSS_ID,omitempty"`
	CompletionCode  string    `json:"completionCode,omitempty"`
}

//nolint:gocritic //impossible to pass pointer
func NewRemedySendUpdateWork(remedyPath string, httpClient *http.Client) RemedySendUpdateWork {
	return RemedySendUpdateWork{
		Name:       "remedy-update-work",
		Input:      make(map[string]string),
		HTTPClient: *httpClient,
		Remedy:     remedyPath,
	}
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendUpdateWork) Inputs() map[string]string {
	return rs.Input
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendUpdateWork) Outputs() map[string]string {
	return make(map[string]string)
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendUpdateWork) IsScenario() bool {
	return false
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendUpdateWork) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return rs.DebugRun(ctx, runCtx)
}

//nolint:dupl, gocritic //its really complex
func (rs RemedySendUpdateWork) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	//nolint:ineffassign, staticcheck //its valid assignment
	ctx, s := trace.StartSpan(ctx, "run_remedy_send_updatework")
	defer s.End()

	ok := false

	defer func() {
		CheckStatusForMetrics(ok)
	}()

	runCtx.AddStep(rs.Name)

	vals := make(map[string]interface{})

	inputs := rs.Model().Inputs
	for _, input := range inputs {
		v, okV := runCtx.GetValue(rs.Input[input.Name])
		if !okV {
			continue
		}

		vals[input.Name] = v
	}

	b, err := json.Marshal(vals)
	if err != nil {
		return err
	}

	m := RemedySendUpdateWorkModel{}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	u, err := url.Parse(rs.Remedy)
	if err != nil {
		return err
	}

	if u.Scheme == "" {
		u.Scheme = httpScheme
	}

	u.Path = "/api/remedy/work/update"

	gatereq, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	gatereq.Header.Add("Content-Type", "application/json")
	gatereq.Header.Add("cache-control", "no-cache")

	resp, err := rs.HTTPClient.Do(gatereq)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return err
	}

	defer resp.Body.Close()

	ok = true

	return err
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendUpdateWork) Next(runCtx *store.VariableStore) string {
	return rs.NextBlock
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendUpdateWork) Model() script.FunctionModel {
	return script.FunctionModel{
		BlockType: script.TypeInternal,
		Title:     "remedy-send-updatework",
		Inputs: []script.FunctionValueModel{
			{
				Name:    "extID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "requestID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "operationID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "status",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "priority",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "planStart",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "deadLine",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "start",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "finish",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "executorGroupID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "executorLogin",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "description",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "mainNIOSSID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "completionCode",
				Type:    script.TypeString,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
