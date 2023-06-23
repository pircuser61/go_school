package pipeline

import (
	c "context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestExecution_Next(t *testing.T) {
	type fields struct {
		Name  string
		Nexts []script.Socket
		State *ExecutionData
	}

	type args struct {
		runCtx *store.VariableStore
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{
			name: "default",
			fields: fields{
				Nexts: []script.Socket{script.DefaultSocket},
				State: &ExecutionData{},
			},
			args: args{
				runCtx: store.NewStore(),
			},
			want: []string(nil),
		},
		{
			name: "test executed",
			fields: fields{
				Nexts: []script.Socket{script.NewSocket("executed", []string{"test-next"})},
				State: &ExecutionData{
					Decision: func() *ExecutionDecision {
						res := ExecutionDecisionExecuted
						return &res
					}(),
				},
			},
			args: args{
				runCtx: store.NewStore(),
			},
			want: []string{"test-next"},
		},
		{
			name: "test edit app",
			fields: fields{
				Nexts: []script.Socket{script.NewSocket("executor_send_edit_app", []string{"test-next"})},
				State: &ExecutionData{
					Decision: func() *ExecutionDecision {
						res := ExecutionDecisionSentEdit
						return &res
					}(),
					EditingApp: nil,
				},
			},
			args: args{
				runCtx: store.NewStore(),
			},
			want: []string{"test-next"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := &GoExecutionBlock{
				Name:    test.fields.Name,
				Sockets: test.fields.Nexts,
				State:   test.fields.State,
			}
			got, _ := block.Next(test.args.runCtx)

			assert.Equal(t, test.want, got)
		})
	}
}

func TestGoExecutionBlock_createGoExecutionBlock(t *testing.T) {
	const (
		example             = "example"
		title               = "title"
		executorsFromSchema = "form_0.user.username;form_1.user.username"
		executorFromSchema  = "form_0.user.username"
	)
	myStorage := makeStorage()

	varStore := store.NewStore()

	varStore.SetValue("form_0.user", map[string]interface{}{
		"username": "test",
		"fullname": "test test test",
	})
	varStore.SetValue("form_1.user", map[string]interface{}{
		"username": "test2",
		"fullname": "test2 test test",
	})

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			Id:           rejectedSocketID,
			Title:        script.RejectSocketTitle,
			NextBlockIds: []string{"next_1"},
		},
	}

	type args struct {
		name   string
		ef     *entity.EriusFunc
		runCtx *BlockRunContext
	}

	tests := []struct {
		name string
		args args
		want *GoExecutionBlock
	}{
		{
			name: "no execution params",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoExecutionID,
					Sockets:   next,
					Input:     nil,
					Output:    nil,
					Params:    nil,
					Title:     title,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want: nil,
		},
		{
			name: "invalid execution params",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoExecutionID,
					Sockets:   next,
					Input:     nil,
					Output:    nil,
					Params:    []byte("{}"),
					Title:     title,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want: nil,
		},
		{
			name: "executors from schema",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
					Storage:           myStorage,
				},
				ef: &entity.EriusFunc{
					BlockType: BlockGoExecutionID,
					Title:     title,
					Sockets:   next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ExecutionParams{
							Type:               script.ExecutionTypeFromSchema,
							Executors:          executorsFromSchema,
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						return r
					}(),
				},
			},
			want: &GoExecutionBlock{
				Name:  example,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					"foo": "bar",
				},
				Sockets: entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					Storage:           myStorage,
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
				},
				State: &ExecutionData{
					WorkType:           "8/5",
					ExecutionType:      script.ExecutionTypeFromSchema,
					Executors:          map[string]struct{}{"test": {}, "test2": {}},
					SLA:                8,
					FormsAccessibility: make([]script.FormAccessibility, 1),
				},
			},
		},
		{
			name: "executor from schema",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
					Storage:           myStorage,
				},
				ef: &entity.EriusFunc{
					BlockType: BlockGoExecutionID,
					Title:     title,
					Sockets:   next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ExecutionParams{
							Type:               script.ExecutionTypeFromSchema,
							Executors:          executorFromSchema,
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						return r
					}(),
				},
			},
			want: &GoExecutionBlock{
				Name:  example,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					"foo": "bar",
				},
				Sockets: entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					Storage:           myStorage,
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
				},
				State: &ExecutionData{
					WorkType:           "8/5",
					ExecutionType:      script.ExecutionTypeFromSchema,
					Executors:          map[string]struct{}{"test": {}},
					SLA:                8,
					FormsAccessibility: make([]script.FormAccessibility, 1),
				},
			},
		},
		{
			name: "load execution state",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&ExecutionData{
							ExecutionType: script.ExecutionTypeUser,
							Executors: map[string]struct{}{
								"tester": {},
							},
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}
						return s
					}(),
				},
				ef: &entity.EriusFunc{
					BlockType: BlockGoExecutionID,
					Title:     title,
					Sockets:   next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ExecutionParams{
							Type:               script.ExecutionTypeUser,
							Executors:          "tester",
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						return r
					}(),
				},
			},
			want: &GoExecutionBlock{
				Name:  example,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					"foo": "bar",
				},
				Sockets: entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&ExecutionData{
							ExecutionType: script.ExecutionTypeUser,
							Executors: map[string]struct{}{
								"tester": {},
							},
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}
						s.Steps = []string{example}
						return s
					}(),
				},
				State: &ExecutionData{
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						"tester": {},
					},
					SLA:                1,
					FormsAccessibility: make([]script.FormAccessibility, 1),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := c.Background()
			got, _, _ := createGoExecutionBlock(ctx, test.args.name, test.args.ef, test.args.runCtx)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestGoExecutionBlock_Update(t *testing.T) {

}
