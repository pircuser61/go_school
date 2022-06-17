package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ptest "gitlab.services.mts.ru/jocasta/pipeliner/internal/handlers/test"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/test"
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
	//defer monkey.UnpatchAll()

	mockDB := test.NewMockDB()

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
				"Output": "Value",
			},
			tp: test.LinearPipelineTestable,
			HandlersExpectedInput: map[string]string{
				"Block1": "{\"Output\":\"Value\"}",
				"Block2": "{\"Output\":\"Value\"}",
				"Block3": "{\"Output\":\"Value\"}",
			},

			PipelinerExpectedOutput: map[string]string{
				"Output": "Value",
			},
			ExpectedRunningSequence: []string{"Block1", "Block2", "Block3"},
		},
		{
			name: "If Pipeline True",
			pipelineInput: map[string]interface{}{
				"Output": "Value",
			},
			tp:                      test.IfPipelineTestable,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "BlockTrue"},
		},
		{
			name: "If Pipeline False",
			pipelineInput: map[string]interface{}{
				"Output": "Unexpected",
			},
			tp:                      test.IfPipelineTestable,
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "BlockFalse"},
		},
		//todo how pipeliner should react to broken function-block?
		//{
		//	name: "Pipeline with broken function block",
		//	pipelineInput: map[string]interface{}{
		//		"Output": "Value",
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
		//{
		//	name: "For Pipeline",
		//	pipelineInput: map[string]interface{}{
		//		"Output": 3,
		//	},
		//	tp:                      test.ForPipelineTestable,
		//	HandlersExpectedInput:   nil,
		//	PipelinerExpectedOutput: nil,
		//	ExpectedRunningSequence: []string{"Block1", "Block2", "Block2", "Block2", "Block3"},
		//},
		{
			name: "Pipeline with pipeline",
			pipelineInput: map[string]interface{}{
				"Output": "Value",
			},
			tp: test.PipelineWithPipelineTestable,
			HandlersExpectedInput: map[string]string{
				"Block1": "{\"Output\":\"Value\"}",
				"Block2": "{\"Output\":\"Value\"}",
				"Block3": "{\"Output\":\"Value\"}",
			},
			PipelinerExpectedOutput: nil,
			ExpectedRunningSequence: []string{"Block1", "Block1", "Block2", "Block3", "Block2"},
		},
		//{
		//	name:                    "ForInFor",
		//	pipelineInput:           nil,
		//	tp:                      test.ForInForPipelineTestable,
		//	HandlersExpectedInput:   nil,
		//	PipelinerExpectedOutput: nil,
		//	ExpectedRunningSequence: []string{"MasGen", "MasGen", "Block1", "Block1", "Block1", "MasGen", "Block1", "Block1", "Block1", "MasGen", "Block1", "Block1", "Block1"},
		//},
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
				"Block3": "{\"Output\":[\"1\",\"2\",\"3\"]}",
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
				ScriptManager: "",
				Remedy:        "",
				FaaS:          FaaSMockServer.URL + "/",
			}

			pipelineRouter := chi.NewRouter()

			pipelineRouter.Route("/", func(r chi.Router) {
				r.Use(LoggerMiddleware(logger.GetLogger(context.Background())))
				r.With(RequestIDMiddleware).Post("/pipeliner/{pipelineID}", ae.RunPipeline)
			})

			pipelinerServer := httptest.NewServer(pipelineRouter)
			defer pipelinerServer.Close()

			pipelineInputBytes, _ := json.Marshal(tt.pipelineInput)

			req, _ := http.NewRequest(
				"POST",
				pipelinerServer.URL+"/pipeliner/"+tt.tp.PipelineUUID.String(),
				bytes.NewReader(pipelineInputBytes))

			resp, err := pipelinerServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

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
	//monkey.PatchInstanceMethod(
	//	reflect.TypeOf(&auth.Client{}),
	//	"CheckGrants",
	//	func(*auth.Client, context.Context, vars.ResourceType, vars.ActionType) (*auth.Grants, error) {
	//		alwaysGrantsAll := &auth.Grants{Allow: true, All: true}
	//
	//		return alwaysGrantsAll, nil
	//	})
	//
	//monkey.PatchInstanceMethod(
	//	reflect.TypeOf(&auth.Client{}),
	//	"Notice",
	//	func(*auth.Client, context.Context, *auth.Notice) error {
	//		return nil
	//	})
}

func Test_scenarioUsage(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		db      db.PipelineStorager
		id      uuid.UUID
		want    []entity.EriusScenario
		wantErr bool
	}{
		{
			name: "err on get pipeline",
			ctx:  context.Background(),
			db: ptest.MockPipelinerStorer{
				Get: func() (*entity.EriusScenario, error) {
					return nil, errors.New("failed")
				},
			},
			wantErr: true,
		},
		{
			name: "err on get worked pipelines",
			ctx:  context.Background(),
			db: ptest.MockPipelinerStorer{
				Get: func() (*entity.EriusScenario, error) {
					return &entity.EriusScenario{}, nil
				},
				Worked: func() ([]entity.EriusScenario, error) {
					return nil, errors.New("failed")
				},
			},
			wantErr: true,
		},
		{
			name: "ok",
			ctx:  context.Background(),
			db: ptest.MockPipelinerStorer{
				Get: func() (*entity.EriusScenario, error) {
					return &entity.EriusScenario{
						Name: "parent",
					}, nil
				},
				Worked: func() ([]entity.EriusScenario, error) {
					return []entity.EriusScenario{
						ptest.Test1(),
						ptest.Test2(),
					}, nil
				},
			},
			want: []entity.EriusScenario{ptest.Test1()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scenarioUsage(tt.ctx, tt.db, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("scenarioUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scenarioUsage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_execVersion(t *testing.T) {
	pipeliner := APIEnv{
		DB:                   test.NewMockDB(),
		ScriptManager:        "",
		Remedy:               "",
		FaaS:                 "",
		SchedulerClient:      nil,
		NetworkMonitorClient: nil,
		HTTPClient:           nil,
		Statistic:            nil,
	}

	t.Run("name", func(t *testing.T) {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
		defer cancel()

		b, err := os.ReadFile("testdata/put_version_extra.json")
		assert.NoError(t, err)
		var p entity.EriusScenario
		err = json.Unmarshal(b, &p)
		assert.NoError(t, err)

		reqId := "123"

		vars := map[string]interface{}{}

		userName := "242"

		if _, _, err := pipeliner.execVersionInternal(ctx, &execVersionInternalParams{
			reqID:         reqId,
			p:             &p,
			vars:          vars,
			syncExecution: true,
			userName:      userName,
		}); err != nil {
			assert.NoError(t, err)
		}
	})

}
