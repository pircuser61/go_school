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

type RemedySendCreateWork struct {
	Name       string
	NextBlock  string
	Input      map[string]string
	HttpClient http.Client
	Remedy     string
}

type RemedySendCreateWorkModel struct {
	WorkType               string    `json:"WorkType,omitempty"`
	ClassificatorCause     string    `json:"ClassificatorCause,omitempty"`
	Status                 string    `json:"Status,omitempty"`
	Priority               string    `json:"Priority,omitempty"`
	ShortDescription       string    `json:"ShortDescription,omitempty"`
	NE                     string    `json:"NE,omitempty"`
	ServiceType            string    `json:"ServiceType,omitempty"`
	ServiceImpactCls       string    `json:"ServiceImpactCls,omitempty"`
	PlanStart              time.Time `json:"PlanStart,omitempty"`
	PlanBegRestrictService time.Time `json:"PlanBegRestrictService,omitempty"`
	PlanEndRestrictService time.Time `json:"PlanEndRestrictService,omitempty"`
	DeadLine               time.Time `json:"DeadLine,omitempty"`
	ExecutorGroupID        string    `json:"ExecutorGroupID,omitempty"`
	ExecutorLogin          string    `json:"ExecutorLogin,omitempty"`
	InitiatorLogin         string    `json:"InitiatorLogin,omitempty"`
	SupervisorLogin        string    `json:"SupervisorLogin,omitempty"`
	SupervisorGroupID      string    `json:"SupervisorGroupID,omitempty"`
	HwSubSystem            string    `json:"HwSubSystem,omitempty"`
	Context                string    `json:"Context,omitempty"`
	Description            string    `json:"Description,omitempty"`
	ExtID                  string    `json:"ExtID,omitempty"`
	IncID                  string    `json:"IncID,omitempty"`
	SIID                   string    `json:"SI_ID,omitempty"`
	InReport               string    `json:"InReport,omitempty"`
	GetSupplier            string    `json:"GetSupplier,omitempty"`
	NotifyService          string    `json:"NotifyService,omitempty"`
	OperationID            string    `json:"OperationID,omitempty"`
	HwRegion               string    `json:"HwRegion,omitempty"`
	MainNIOSSID            string    `json:"Main_NIOSS_ID,omitempty"`
	Module                 string    `json:"Module,omitempty"`
	FileList               FileItem  `json:"FileList,omitempty"`
}

func NewRemedySendCreateWork(remedyPath string, httpClient *http.Client) RemedySendCreateWork {
	return RemedySendCreateWork{
		Name:       "remedy_create_work",
		Input:      make(map[string]string),
		HttpClient: *httpClient,
		Remedy:     remedyPath,
	}
}

func (rs RemedySendCreateWork) Inputs() map[string]string {
	return rs.Input
}

func (rs RemedySendCreateWork) Outputs() map[string]string {
	return make(map[string]string)
}

func (rs RemedySendCreateWork) IsScenario() bool {
	return false
}

func (rs RemedySendCreateWork) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return rs.DebugRun(ctx, runCtx)
}

func (rs RemedySendCreateWork) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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

	m := RemedySendCreateWorkModel{}

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

	u.Path = path.Join(rs.Remedy, "/api/remedy/work/create")

	gatereq, err := http.NewRequest("Post", u.String(), bytes.NewBuffer(b))
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

func (rs RemedySendCreateWork) Next() string {
	return rs.NextBlock
}

func (rs RemedySendCreateWork) Model() script.FunctionModel {
	return script.FunctionModel{
		BlockType: script.TypeInternal,
		Title:     "remedy-send-creatework",
		Inputs: []script.FunctionValueModel{
			{
				Name:    "workType",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "classificatorCause",
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
				Name:    "shortDescription",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "ne",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "serviceType",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "serviceImpactCls",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "planStart",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "planBegRestrictService",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "planEndRestrictService",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "deadLine",
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
				Name:    "initiatorLogin",
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
				Name:    "hwSubSystem",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "context",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "description",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "extID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "incID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "siID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "inReport",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "getSupplier",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "notifyService",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "operationID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "hwRegion",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "mainNIOSSID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "module",
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
