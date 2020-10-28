package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"go.opencensus.io/trace"
)

type RemedySendUpdateProblem struct {
	Name       string
	NextBlock  string
	Input      map[string]string
	HttpClient http.Client
	Remedy     string
}

type RemedySendUpdateProblemModel struct{
	OperationID              string       `json:"operationID,omitempty"`
	ExtID                    string       `json:"extID,omitempty"`
	RequestID                string       `json:"request_id,omitempty"`
	Source                   string       `json:"source,omitempty"`
	Region                   string       `json:"region,omitempty"`
	Status                   int          `json:"status,omitempty"`
	Priority                 int          `json:"Priority,omitempty"`
	ClassificatorDescription string       `json:"classificatordescription,omitempty"`
	ServiceImpactCls         string       `json:"serviceImpactCls,omitempty"`
	SolutionCode             int          `json:"solutioncode,omitempty"`
	FlagInvestment           int          `json:"flaginvestment,omitempty"`
	ClosureCode              int          `json:"closurecode,omitempty"`
	Description              string       `json:"description,omitempty"`
	ClassificatorCause       string       `json:"classificatorcause,omitempty"`
	ClassificatorSolution    string       `json:"classificatorsolution,omitempty"`
	Solution                 string       `json:"solution,omitempty"`
	ResponsibilityZone       string       `json:"responsibility_zone,omitempty"`
	EventTime                time.Time    `json:"eventtime,omitempty"`
	FixTime                  time.Time    `json:"fixtime,omitempty"`
	Deadline                 time.Time    `json:"deadline,omitempty"`
	SolutionPlanTime         time.Time    `json:"solutionplantime,omitempty"`
	InitiatorLogin           string       `json:"initiator_login,omitempty"`
	ExecutorLogin            string       `json:"executor_login,omitempty"`
	ExecutorGroupID          string       `json:"executor_group_id,omitempty"`
	SupervisorLogin          string       `json:"supervisor_login,omitempty"`
	SupervisorGroupID        string       `json:"supervisor_group_id,omitempty"`
	NENiossID                string       `json:"ne_nioss_id,omitempty"`
	NESubsystem              string       `json:"ne_subsystem,omitempty"`
}

func NewRemedySendUpdateProblem(remedyPath string, httpClient *http.Client) RemedySendUpdateProblem {
	return RemedySendUpdateProblem{
		Name:       "remedy-update-problem",
		Input:      make(map[string]string),
		HttpClient: *httpClient,
		Remedy:     remedyPath,
	}
}

func (rs RemedySendUpdateProblem) Inputs() map[string]string {
	return rs.Input
}

func (rs RemedySendUpdateProblem) Outputs() map[string]string {
	return make(map[string]string)
}

func (rs RemedySendUpdateProblem) IsScenario() bool {
	return false
}

func (rs RemedySendUpdateProblem) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return rs.DebugRun(ctx, runCtx)
}

func (rs RemedySendUpdateProblem) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_remedy_send")
	defer s.End()

	ok := false

	defer func() {
		if ok {
			metrics.Stats.RemedyPushes.Ok.SetToCurrentTime()
		} else {
			metrics.Stats.RemedyPushes.Fail.SetToCurrentTime()
		}

		errPush := metrics.Pusher.Push()
		if errPush != nil {
			fmt.Printf("can't push: %s\n", errPush.Error())
		}
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

	m := RemedySendUpdateProblemModel{}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	u, err := url.Parse(rs.Remedy)
	if err != nil {
		return err
	}

	if u.Scheme == "" {
		u.Scheme = "http"
	}

	u.Path = path.Join(rs.Remedy, "/api/remedy/problem/update")

	gatereq, err := http.NewRequest("Put", u.String(), bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	gatereq.Header.Add("Content-Type", "application/json")
	gatereq.Header.Add("cache-control", "no-cache")

	resp, err := rs.HttpClient.Do(gatereq)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return err
	}

	defer resp.Body.Close()

	return err
}

func (rs RemedySendUpdateProblem) Next() string {
	return rs.NextBlock
}

func (rs RemedySendUpdateProblem) Model() script.FunctionModel {
	return script.FunctionModel{
		BlockType: script.TypeInternal,
		Title:     "remedy-send-updateproblem",
		Inputs: []script.FunctionValueModel{
			{
				Name:    "operationID",
				Type:    script.TypeString,
				Comment: "",
			},
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
				Name:    "source",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "region",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "status",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "priority",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "classificatorDescription",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "serviceImpactCls",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "solutionCode",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "flagInvestment",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "closureCode",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "description",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "classificatorCause",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "classificatorSolution",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "solution",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "responsibilityZone",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "eventTime",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "fixTime",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "deadline",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "solutionPlanTime",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "initiatorLogin",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "executorLogin",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "executorGroupID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "supervisorLogin",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "supervisorGroupID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neNiossID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neSubsystem",
				Type:    script.TypeString,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
