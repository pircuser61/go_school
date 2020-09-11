package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"gitlab.services.mts.ru/erius/pipeliner/internal/test"
	"gitlab.services.mts.ru/libs/logger"
)

func AddWithStop(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "with_stop", true)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func TestAPIEnv_RunPipeline(t *testing.T) {
	mockDB := test.NewMockDB()
	log := logger.CreateLogger(nil)

	tests := []struct {
		name string

		pipelineInput           map[string]interface{}
		tp                      test.TestablePipeline
		HandlersExpectedInput   map[string]string
		PiplinerExpectedOutput  map[string]string
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

			PiplinerExpectedOutput: map[string]string{
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
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"Block1", "BlockTrue"},
		},
		{
			name: "If Pipeline False",
			pipelineInput: map[string]interface{}{
				"Input": "Unexpected",
			},
			tp:                      test.IfPipelineTestable,
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"Block1", "BlockFalse"},
		},
		//todo how pipeliner should react to broken function-block?
		//{
		//	name: "Pipeline with broken function block",
		//	pipelineInput: map[string]interface{}{
		//		"Input": "Value",
		//	},
		//	pipelineUUID: test.LinearPipelineUUID,
		//	FunctionHandlers: map[string]http.HandlerFunc{
		//		"Block1": LinearPipelineBlock,
		//		"Block2": BrokenBlock,
		//		"Block3": LinearPipelineBlock,
		//	},
		//	HandlersExpectedInput:   nil,
		//	PiplinerExpectedOutput:  nil,
		//	ExpectedRunningSequence: []string{"Block1", "Block2"},
		//},
		{
			name: "For Pipeline",
			pipelineInput: map[string]interface{}{
				"Input": 3,
			},
			tp:                      test.ForPipelineTestable,
			HandlersExpectedInput:   nil,
			PiplinerExpectedOutput:  nil,
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
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"Block1", "Block1", "Block2", "Block3", "Block2"},
		},
		{
			name:                    "ForInFor",
			pipelineInput:           nil,
			tp:                      test.ForInForPipelineTestable,
			HandlersExpectedInput:   nil,
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"MasGen", "MasGen", "Block1", "Block1", "Block1", "MasGen", "Block1", "Block1", "Block1", "MasGen", "Block1", "Block1", "Block1"},
		},
		{
			name:                    "Strings equal Pipeline True",
			tp:                      test.StringsEqualsPipelineTrueTestable,
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "BlockTrue"},
		},
		{
			name:                    "Strings equal Pipeline False",
			tp:                      test.StringsEqualsPipelineFalseTestable,
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "BlockFalse"},
		},
		{
			name:          "Connector Pipeline",
			pipelineInput: nil,
			tp:            test.ConnectorPipelineTestable,
			HandlersExpectedInput: map[string]string{
				"Block3": "{\"Input\":[\"1\",\"2\",\"3\"]}",
			},
			PiplinerExpectedOutput:  nil,
			ExpectedRunningSequence: []string{"Block1", "Block2", "Block3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedBlockIndex := 0
			FaaSMockServer := httptest.NewServer(chi.NewRouter().Route("/", func(r chi.Router) {
				r.Post("/function/{funcName}", func(w http.ResponseWriter, rq *http.Request) {
					funcName := chi.URLParam(rq, "funcName")
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

			resp, _ := pipelinerServer.Client().Do(req)
			respBytes, _ := ioutil.ReadAll(resp.Body)

			time.Sleep(1 * time.Second)

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
