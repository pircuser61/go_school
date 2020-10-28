package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"go.opencensus.io/trace"
	"net/http"
	"net/url"
	"path"
	"time"
)

type RemedySendUpdateWork struct {
	Name       string
	NextBlock  string
	Input      map[string]string
	HttpClient http.Client
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
	FileList        FileItem  `json:"FileList,omitempty"`
}

func NewRemedySendUpdateWork(remedyPath string, httpClient *http.Client) RemedySendUpdateWork {
	return RemedySendUpdateWork{
		Name:       "remedy_update_work",
		Input:      make(map[string]string),
		HttpClient: *httpClient,
		Remedy:     remedyPath,
	}
}

func (rs RemedySendUpdateWork) Inputs() map[string]string {
	return rs.Input
}

func (rs RemedySendUpdateWork) Outputs() map[string]string {
	return make(map[string]string)
}

func (rs RemedySendUpdateWork) IsScenario() bool {
	return false
}

func (rs RemedySendUpdateWork) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return rs.DebugRun(ctx, runCtx)
}

func (rs RemedySendUpdateWork) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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
		u.Scheme = "http"
	}

	u.Path = path.Join(rs.Remedy, "/api/remedy/work/update")

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

func (rs RemedySendUpdateWork) Next() string {
	return rs.NextBlock
}

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
			{
				Name:    "fileIndex",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "fileTimestamp",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "fileURL",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "fileName",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "fileSize",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "fileAuthor",
				Type:    script.TypeString,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
