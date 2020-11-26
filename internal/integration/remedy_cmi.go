package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

type RemedySendCreateMI struct {
	Name       string
	NextBlock  string
	Input      map[string]string
	HTTPClient http.Client
	Remedy     string
}

type RemedySendCreateMIModel struct {
	ExtID               string    `json:"ext_id,omitempty"`
	OperationID         string    `json:"operation_id,omitempty"`
	ExecutorLogin       string    `json:"executor_login,omitempty"`
	ExecutorGroupID     string    `json:"executor_group_id,omitempty"`
	PlaceAddress        string    `json:"place_address,omitempty"`
	Theme               string    `json:"theme,omitempty"`
	Scale               int       `json:"scale,omitempty"`
	Influence           int       `json:"influence,omitempty"`
	Urgency             int       `json:"urgency,omitempty"`
	Subject             string    `json:"subject,omitempty"`
	Regtime             time.Time `json:"regtime,omitempty"`
	Region              string    `json:"region,omitempty"`
	BusDesc             string    `json:"bus_desc,omitempty"`
	NiossID             string    `json:"nioss_id,omitempty"`
	NE                  string    `json:"ne,omitempty"`
	ObjPriority         int       `json:"obj_priority,omitempty"`
	ImpactDesc          string    `json:"impact_desc,omitempty"`
	SupervisorGroupID   string    `json:"supervisor_group_id,omitempty"`
	SupervisorLogin     string    `json:"supervisor_login,omitempty"`
	InitiatorLogin      string    `json:"initiator_login,omitempty"`
	StopServDat         time.Time `json:"stop_serv_dat,omitempty"`
	ShortDesc           string    `json:"short_desc,omitempty"`
	ServiceSiebel       string    `json:"service_siebel,omitempty"`
	RespZone            string    `json:"resp_zone,omitempty"`
	KPI                 int       `json:"kpi,omitempty"`
	ExtDesc             string    `json:"ext_desc,omitempty"`
	AlarmMessage        string    `json:"alarm_message,omitempty"`
	NEAlias             string    `json:"ne_alias,omitempty"`
	MRClusterF1         int       `json:"mr_cluster_f1,omitempty"`
	MRClusterF2         int       `json:"mr_cluster_f2,omitempty"`
	MRClusterF3         int       `json:"mr_cluster_f3,omitempty"`
	MRClusterF4         int       `json:"mr_cluster_f4,omitempty"`
	DeadlineExceedCause string    `json:"deadline_exceed_cause,omitempty"`
	NEVendor            string    `json:"ne_vendor,omitempty"`
	NESubsystem         string    `json:"ne_subsystem,omitempty"`
	NEType              string    `json:"ne_type,omitempty"`
	InReport            int       `json:"inreport,omitempty"`
	KnownProblem        int       `json:"known_problem,omitempty"`
	NEName              string    `json:"ne_name,omitempty"`
	NESegment           string    `json:"ne_segment,omitempty"`
	NETimeRoad          int       `json:"ne_time_road,omitempty"`
	NEAddress           string    `json:"ne_address,omitempty"`
	NESite              string    `json:"ne_site,omitempty"`
	NESubtype           string    `json:"ne_subtype,omitempty"`
	NotifyService       int       `json:"notify_service,omitempty"`
	NotifyServiceTime   time.Time `json:"notify_service_time,omitempty"`
	NEServiceType       string    `json:"ne_service_type,omitempty"`
	SiebelTimeRoad      int       `json:"siebel_time_road,omitempty"`
	SiebelScale2        int       `json:"siebel_scale2,omitempty"`
	TermSolution        time.Time `json:"term_solution,omitempty"`
}

func NewRemedySendCreateMI(remedyPath string, httpClient *http.Client) RemedySendCreateMI {
	return RemedySendCreateMI{
		Name:       "remedy-create-mi",
		Input:      make(map[string]string),
		HTTPClient: *httpClient,
		Remedy:     remedyPath,
	}
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendCreateMI) Inputs() map[string]string {
	return rs.Input
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendCreateMI) Outputs() map[string]string {
	return make(map[string]string)
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendCreateMI) IsScenario() bool {
	return false
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendCreateMI) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return rs.DebugRun(ctx, runCtx)
}

//nolint:dupl, gocritic //its really complex
func (rs RemedySendCreateMI) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	//nolint:ineffassign, staticcheck //its valid assignment
	ctx, s := trace.StartSpan(ctx, "run_remedy_send_createmi")
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

	m := RemedySendCreateMIModel{}

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

	u.Path = "/api/remedy/incident/create"

	gatereq, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(b))
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

func CheckStatusForMetrics(ok bool) {
	if ok {
		metrics.Stats.RemedyPushes.Ok.SetToCurrentTime()
	} else {
		metrics.Stats.RemedyPushes.Fail.SetToCurrentTime()
	}

	errPush := metrics.Pusher.Add()
	if errPush != nil {
		fmt.Printf("can't push: %s\n", errPush.Error())
	}
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendCreateMI) Next() string {
	return rs.NextBlock
}

//nolint:gocritic //impossible to pass pointer
func (rs RemedySendCreateMI) Model() script.FunctionModel {
	return script.FunctionModel{
		BlockType: script.TypeInternal,
		Title:     "remedy-send-createmi",
		Inputs: []script.FunctionValueModel{
			{
				Name:    "extID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "operationID",
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
				Name:    "placeAddress",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "theme",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "scale",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "influence",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "urgency",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "subject",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "regtime",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "region",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "busDesc",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "niossID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "ne",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "objPriority",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "impactDesc",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "supervisorGroupID",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "supervisorLogin",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "initiatorLogin",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "stopServDat",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "shortDesc",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "serviceSiebel",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "respZone",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "kpi",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "extDesc",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "alarmMessage",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neAlias",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "mrClusterF1",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "mrClusterF2",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "mrClusterF3",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "mrClusterF4",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "deadlineExceedCause",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neVendor",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neSubsystem",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neType",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "inReport",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "knownProblem",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "neName",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neSegment",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neTimeRoad",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "neAddress",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neSite",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neSubtype",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "notifyService",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "notifyServiceTime",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "neServiceType",
				Type:    script.TypeString,
				Comment: "",
			},
			{
				Name:    "siebelTimeRoad",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "siebelScale2",
				Type:    script.TypeNumber,
				Comment: "",
			},
			{
				Name:    "termSolution",
				Type:    script.TypeString,
				Comment: "",
			},
		},
		Outputs:   nil,
		NextFuncs: []string{script.Next},
		ShapeType: script.ShapeIntegration,
	}
}
