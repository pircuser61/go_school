package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"bou.ke/monkey"

	"gitlab.services.mts.ru/erius/admin/pkg/auth"

	"gitlab.services.mts.ru/erius/admin/pkg/vars"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"

	"github.com/stretchr/testify/assert"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"

	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"

	"github.com/go-chi/chi"
	"gitlab.services.mts.ru/libs/logger"

	"gitlab.services.mts.ru/erius/pipeliner/internal/test"
)

func AddWithStop(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "with_stop", true)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func TestAPIEnv_RunPipeline(t *testing.T) {
	patchAuthClient()
	defer monkey.UnpatchAll()

	mockDB := test.NewMockDB()
	log := logger.CreateLogger(nil)

	tests := []struct {
		name string

		pipelineInput           map[string]interface{}
		tp                      test.TestablePipeline
		HandlersExpectedInput   map[string]string
		PipelinerExpectedOutput map[string]string
		ExpectedRunningSequence []string
	}{
		{
			name: "Linear pipeline",

			pipelineInput: map[string]interface{}{
				"Input": "Value",
			},
			tp: test.LinearPipelineTestable,
			HandlersExpectedInput: map[string]string{
				"Block1": "{\"Input\":\"Value\"}",
				"Block2": "{\"Input\":\"Value\"}",
				"Block3": "{\"Input\":\"Value\"}",
			},

			PipelinerExpectedOutput: map[string]string{
				"Input": "Value",
			},
			ExpectedRunningSequence: []string{"Block1", "Block2", "Block3"},
		},
		{
			name: "If Pipeline True",
			pipelineInput: map[string]interface{}{
				"Input": "Value",
			},
			tp:                      test.IfPipelineTestable,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "BlockTrue"},
		},
		{
			name: "If Pipeline False",
			pipelineInput: map[string]interface{}{
				"Input": "Unexpected",
			},
			tp:                      test.IfPipelineTestable,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "BlockFalse"},
		},
		//todo how pipeliner should react to broken function-block?
		//{
		//	name: "Pipeline with broken function block",
		//	pipelineInput: map[string]interface{}{
		//		"Input": "Value",
		//	},
		//	pipelineUUID: test.linearPipelineUUID,
		//	FunctionHandlers: map[string]http.HandlerFunc{
		//		"Block1": linearPipelineBlock,
		//		"Block2": BrokenBlock,
		//		"Block3": linearPipelineBlock,
		//	},
		//	HandlersExpectedInput:   nil,
		//	PipelinerExpectedOutput:  nil,
		//	ExpectedRunningSequence: []string{"Block1", "Block2"},
		//},
		{
			name: "For Pipeline",
			pipelineInput: map[string]interface{}{
				"Input": 3,
			},
			tp:                      test.ForPipelineTestable,
			HandlersExpectedInput:   nil,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "Block2", "Block2", "Block3"},
		},
		{
			name: "Pipeline with pipeline",
			pipelineInput: map[string]interface{}{
				"Input": "Value",
			},
			tp: test.PipelineWithPipelineTestable,
			HandlersExpectedInput: map[string]string{
				"Block1": "{\"Input\":\"Value\"}",
				"Block2": "{\"Input\":\"Value\"}",
				"Block3": "{\"Input\":\"Value\"}",
			},
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "Block1", "Block2", "Block3", "Block2"},
		},
		{
			name:                    "ForInFor",
			pipelineInput:           nil,
			tp:                      test.ForInForPipelineTestable,
			HandlersExpectedInput:   nil,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"MasGen", "MasGen", "Block1", "Block1", "Block1", "MasGen", "Block1", "Block1", "Block1", "MasGen", "Block1", "Block1", "Block1"},
		},
		{
			name:                    "Strings equal Pipeline True",
			tp:                      test.StringsEqualsPipelineTrueTestable,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "BlockTrue"},
		},
		{
			name:                    "Strings equal Pipeline False",
			tp:                      test.StringsEqualsPipelineFalseTestable,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "BlockFalse"},
		},
		{
			name:          "Connector Pipeline",
			pipelineInput: nil,
			tp:            test.ConnectorPipelineTestable,
			HandlersExpectedInput: map[string]string{
				"Block3": "{\"Input\":[\"1\",\"2\",\"3\"]}",
			},
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "Block3"},
		},
	}

	monitoring.Setup("http://localhost:9000/api/monitoring/v1/pipeliner", http.DefaultClient)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedBlockIndex := 0
			FaaSMockServer := httptest.NewServer(chi.NewRouter().Route("/", func(r chi.Router) {
				r.Post("/function/{onApprovedVersions}", func(w http.ResponseWriter, rq *http.Request) {
					funcName := chi.URLParam(rq, "onApprovedVersions")
					handler, ok := tt.tp.FunctionHandlers[funcName]
					if !ok {
						t.Fatalf("No such func %s", funcName)
					}

					if expectedBlockIndex >= len(tt.ExpectedRunningSequence) {
						t.Errorf("Unexpected running of function block %s", funcName)
					}

					if funcName != tt.ExpectedRunningSequence[expectedBlockIndex] {
						t.Errorf("Function block ran = %s, function block want = %s",
							funcName, tt.ExpectedRunningSequence[expectedBlockIndex])
					}

					t.Log("Running " + funcName)

					inputBytes, _ := ioutil.ReadAll(rq.Body)

					if tt.HandlersExpectedInput != nil {
						if _, ok = tt.HandlersExpectedInput[funcName]; ok && (string(inputBytes) != tt.HandlersExpectedInput[funcName]) && tt.HandlersExpectedInput[funcName] != "" {
							t.Errorf("unexpected input for function %s; expected=%s, actual=%s",
								funcName, tt.HandlersExpectedInput[funcName], string(inputBytes))
						}
					}

					rq.Body = ioutil.NopCloser(bytes.NewBuffer(inputBytes))

					handler(w, rq)

					expectedBlockIndex += 1
				})
			}))
			defer FaaSMockServer.Close()

			ae := &APIEnv{
				DB:            mockDB,
				Logger:        log,
				ScriptManager: "",
				FaaS:          FaaSMockServer.URL + "/",
				AuthClient:    &auth.Client{},
			}

			pipelineRouter := chi.NewRouter()

			// pipelineRouter.Use(AddWithStop)
			pipelineRouter.Route("/", func(r chi.Router) {
				r.With(SetRequestID).Post("/pipeliner/{pipelineID}", ae.RunPipeline)
			})

			pipelinerServer := httptest.NewServer(pipelineRouter)
			defer pipelinerServer.Close()

			pipelineInputBytes, _ := json.Marshal(tt.pipelineInput)

			req, _ := http.NewRequest(
				"POST",
				pipelinerServer.URL+"/pipeliner/"+tt.tp.PipelineUUID.String(),
				bytes.NewReader(pipelineInputBytes))

			resp, err := pipelinerServer.Client().Do(req)
			_ = err
			respBytes, _ := ioutil.ReadAll(resp.Body)

			time.Sleep(5 * time.Second)

			var httpResp struct {
				StatusCode int `json:"status_code"`
			}

			_ = json.Unmarshal(respBytes, &httpResp)

			if httpResp.StatusCode != 200 {
				t.Errorf("Pipeliner error")
			}

			if tt.ExpectedRunningSequence != nil && expectedBlockIndex != len(tt.ExpectedRunningSequence) {
				t.Errorf("Pipeline didn't run functions")
			}
		})
	}
}

func patchAuthClient() {
	// patch auth client
	monkey.PatchInstanceMethod(
		reflect.TypeOf(&auth.Client{}),
		"CheckGrants",
		func(*auth.Client, context.Context, vars.ResourceType, vars.ActionType) (*auth.Grants, error) {
			alwaysGrantsAll := &auth.Grants{Allow: true, All: true}

			return alwaysGrantsAll, nil
		})

	monkey.PatchInstanceMethod(
		reflect.TypeOf(&auth.Client{}),
		"Notice",
		func(*auth.Client, context.Context, *auth.Notice) error {
			return nil
		})
}

func newUUID(val string) uuid.UUID {
	res, _ := uuid.Parse(val)
	return res
}

func Test_filter(t *testing.T) {
	tests := []struct {
		name  string
		items []entity.EriusScenarioInfo
		isAll bool
		keys  map[string]struct{}
		want  []entity.EriusScenarioInfo
	}{
		{
			name: "ok with all",
			items: []entity.EriusScenarioInfo{
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf1")},
			},
			want: []entity.EriusScenarioInfo{
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf1")},
			},
			isAll: true,
		},
		{
			name: "ok with keys",
			items: []entity.EriusScenarioInfo{
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf1")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf2")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf3")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf4")},
			},
			keys: map[string]struct{}{
				"42bdafca-dce8-4c3d-84c6-4971854d1cf1": {},
				"42bdafca-dce8-4c3d-84c6-4971854d1cf3": {},
			},
			want: []entity.EriusScenarioInfo{
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf1")},
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf3")},
			},
		},
		{
			name:  "return empty if empty items",
			items: []entity.EriusScenarioInfo{},
			keys: map[string]struct{}{
				"42bdafca-dce8-4c3d-84c6-4971854d1cf1": {},
				"42bdafca-dce8-4c3d-84c6-4971854d1cf3": {},
			},
			want: make([]entity.EriusScenarioInfo, 0),
		},
		{
			name: "return input slice if nil map",
			items: []entity.EriusScenarioInfo{
				{VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			},
			keys: nil,
			want: []entity.EriusScenarioInfo{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, filterVersionsByID(tt.items, tt.isAll, tt.keys), "%v", tt.name)
		})
	}
}

func Test_authParametersByPipelineStatus(t *testing.T) {
	tests := []struct {
		name         string
		p            entity.EriusScenario
		wantResource vars.ResourceType
		wantAction   vars.ActionType
		wantID       string
	}{
		{
			name:         "draft",
			p:            entity.EriusScenario{Status: db.StatusDraft, VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			wantResource: vars.PipelineVersion,
			wantAction:   vars.Own,
			wantID:       "42bdafca-dce8-4c3d-84c6-4971854d1cf0",
		},
		{
			name:         "on approve",
			p:            entity.EriusScenario{Status: db.StatusOnApprove, VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			wantResource: vars.PipelineVersion,
			wantAction:   vars.Own,
			wantID:       "42bdafca-dce8-4c3d-84c6-4971854d1cf0",
		},
		{
			name:         "approved",
			p:            entity.EriusScenario{Status: db.StatusApproved, ID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			wantResource: vars.Pipeline,
			wantAction:   vars.Update,
			wantID:       "42bdafca-dce8-4c3d-84c6-4971854d1cf0",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotResource, gotAction, gotID := authUpdateParametersByPipelineStatus(&tt.p)

			assert.Equal(t, tt.wantResource, gotResource, "%v", tt.name)
			assert.Equal(t, tt.wantAction, gotAction, "%v", tt.name)
			assert.Equal(t, tt.wantID, gotID, "%v", tt.name)
		})
	}
}

func Test_authDeleteParametersByPipelineStatus(t *testing.T) {
	tests := []struct {
		name         string
		p            entity.EriusScenario
		wantResource vars.ResourceType
		wantAction   vars.ActionType
		wantID       string
	}{
		{
			name:         "draft",
			p:            entity.EriusScenario{Status: db.StatusDraft, VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			wantResource: vars.PipelineVersion,
			wantAction:   vars.Own,
			wantID:       "42bdafca-dce8-4c3d-84c6-4971854d1cf0",
		},
		{
			name:         "on approve",
			p:            entity.EriusScenario{Status: db.StatusOnApprove, VersionID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			wantResource: vars.Pipeline,
			wantAction:   vars.Delete,
			wantID:       "42bdafca-dce8-4c3d-84c6-4971854d1cf0",
		},
		{
			name:         "approved",
			p:            entity.EriusScenario{Status: db.StatusApproved, ID: newUUID("42bdafca-dce8-4c3d-84c6-4971854d1cf0")},
			wantResource: vars.Pipeline,
			wantAction:   vars.Delete,
			wantID:       "42bdafca-dce8-4c3d-84c6-4971854d1cf0",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotResource, gotAction, gotID := authDeleteParametersByPipelineStatus(&tt.p)

			assert.Equal(t, tt.wantResource, gotResource, "%v", tt.name)
			assert.Equal(t, tt.wantAction, gotAction, "%v", tt.name)
			assert.Equal(t, tt.wantID, gotID, "%v", tt.name)
		})
	}
}
