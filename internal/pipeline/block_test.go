package pipeline

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

func newStoreWithData(data map[string]interface{}) *store.VariableStore {
	s := store.NewStore()

	for key, val := range data {
		s.SetValue(key, val)
	}

	return s
}

func storeContainsData(s *store.VariableStore, data map[string]interface{}) bool {
	st, _ := s.GrabStorage()

	for key, valExp := range data {
		if val, ok := st[key]; !ok || val != valExp {
			return false
		}
	}

	return true
}

var (
	RunOnlyFunction = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(200)
	})

	WithOutputFunction = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("{\"sOutput\":\"sOutputValue\"}"))
	})
)

func TestFunctionBlock_Run(t *testing.T) {
	type fields struct {
		Name           string
		FunctionName   string
		FunctionInput  map[string]string
		FunctionOutput map[string]string
		NextStep       string
		runURL         string
	}
	type args struct {
		ctx    context.Context
		runCtx *store.VariableStore
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool

		funcHandler http.HandlerFunc

		expectedInput string

		expectedGlobalValues map[string]interface{}
	}{
		{
			name: "RunOnly block",
			fields: fields{
				Name:           "RunOnlyBlock",
				FunctionName:   "RunOnlyFunction",
				FunctionInput:  nil,
				FunctionOutput: nil,
				NextStep:       "",
				runURL:         "",
			},
			args: args{
				ctx:    context.Background(),
				runCtx: store.NewStore(),
			},
			wantErr:     false,
			funcHandler: RunOnlyFunction,
		},
		{
			name: "Block with input/output",
			fields: fields{
				Name:         "WithOutputBlock",
				FunctionName: "WithOutputFunction",
				FunctionInput: map[string]string{
					"sInput": "global.sInput",
				},
				FunctionOutput: map[string]string{
					"sOutput": "global.sOutput",
				},
				NextStep: "",
				runURL:   "",
			},
			args: args{
				ctx:    context.Background(),
				runCtx: newStoreWithData(map[string]interface{}{"global.sInput": "sInputValue"}),
			},
			wantErr: false,

			expectedInput: "{\"sInput\":\"sInputValue\"}",

			funcHandler: WithOutputFunction,

			expectedGlobalValues: map[string]interface{}{
				"global.sOutput": "sOutputValue",
			},
		},
	}
	for _, tt := range tests {
		wasRan := false
		server := httptest.NewServer(chi.NewRouter().Route("/", func(r chi.Router) {
			r.Post("/{functionName}", func(w http.ResponseWriter, rq *http.Request) {
				t.Log("Running " + tt.fields.FunctionName)
				if tt.expectedInput != "" {
					inputBytes, _ := ioutil.ReadAll(rq.Body)
					if string(inputBytes) != tt.expectedInput {
						t.Errorf("Got bad input: expected = %s, actual = %s", tt.expectedInput, string(inputBytes))
					}
				}
				tt.funcHandler(w, rq)
				wasRan = true
			})
		}))

		t.Run(tt.name, func(t *testing.T) {
			fb := &FunctionBlock{
				Name:           tt.fields.Name,
				FunctionName:   tt.fields.FunctionName,
				FunctionInput:  tt.fields.FunctionInput,
				FunctionOutput: tt.fields.FunctionOutput,
				NextStep:       tt.fields.NextStep,
				runURL:         server.URL + "/%s",
			}
			if err := fb.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !wasRan {
				t.Errorf("Function %s was not ran", tt.fields.FunctionName)
			}
			if !storeContainsData(tt.args.runCtx, tt.expectedGlobalValues) {
				t.Errorf("Store does not conatins required data")
			}
		})

		server.Close()
	}
}
