package pipeline

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"

	"github.com/stretchr/testify/assert"

	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestBlockRunContext_excludeHiddenFormFields(t *testing.T) {
	desc := orderedmap.New()
	desc.Set("field_1", "value1")
	desc.Set("field_2", "value2")
	desc.Set("field_3", 1)
	desc.Set("imya", "Eva")
	desc.Set("snils", "123423123")
	desc.Set("familiya", "Ebaklan")

	want := orderedmap.New()
	want.Set("field_1", "value1")
	want.Set("familiya", "Ebaklan")

	state := make(map[string]json.RawMessage)
	state["form_0"] = []byte(`
		{"hidden_fields": ["field_2", "field_3", "imya", "snils"]}`)

	type fields struct {
		TaskID               uuid.UUID
		WorkNumber           string
		ClientID             string
		PipelineID           uuid.UUID
		VersionID            uuid.UUID
		WorkTitle            string
		Initiator            string
		IsTest               bool
		CustomTitle          string
		NotifName            string
		Delegations          human_tasks.Delegations
		VarStore             *store.VariableStore
		UpdateData           *script.BlockUpdateData
		CurrBlockStartTime   time.Time
		skipNotifications    bool
		skipProduce          bool
		Services             RunContextServices
		BlockRunResults      *BlockRunResults
		TaskSubscriptionData TaskSubscriptionData
	}
	type args struct {
		formName string
		desc     orderedmap.OrderedMap
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    orderedmap.OrderedMap
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success",
			fields: fields{
				TaskID:      uuid.UUID{},
				WorkNumber:  "1",
				ClientID:    "2",
				PipelineID:  uuid.UUID{},
				VersionID:   uuid.UUID{},
				WorkTitle:   "3",
				Initiator:   "4",
				CustomTitle: "5",
				NotifName:   "6",
				VarStore: &store.VariableStore{
					State:      state,
					StopPoints: store.StopPoints{},
				},
				CurrBlockStartTime:   time.Time{},
				Services:             RunContextServices{},
				TaskSubscriptionData: TaskSubscriptionData{},
			},
			args: args{
				formName: "form_0",
				desc:     *desc,
			},
			want:    *want,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCtx := &BlockRunContext{
				TaskID:               tt.fields.TaskID,
				WorkNumber:           tt.fields.WorkNumber,
				ClientID:             tt.fields.ClientID,
				PipelineID:           tt.fields.PipelineID,
				VersionID:            tt.fields.VersionID,
				WorkTitle:            tt.fields.WorkTitle,
				Initiator:            tt.fields.Initiator,
				IsTest:               tt.fields.IsTest,
				CustomTitle:          tt.fields.CustomTitle,
				NotifName:            tt.fields.NotifName,
				Delegations:          tt.fields.Delegations,
				VarStore:             tt.fields.VarStore,
				UpdateData:           tt.fields.UpdateData,
				CurrBlockStartTime:   tt.fields.CurrBlockStartTime,
				skipNotifications:    tt.fields.skipNotifications,
				skipProduce:          tt.fields.skipProduce,
				Services:             tt.fields.Services,
				BlockRunResults:      tt.fields.BlockRunResults,
				TaskSubscriptionData: tt.fields.TaskSubscriptionData,
			}
			got, err := runCtx.excludeHiddenFormFields(tt.args.formName, tt.args.desc)
			if !tt.wantErr(t, err, fmt.Sprintf("excludeHiddenFormFields(%v, %v)", tt.args.formName, tt.args.desc)) {
				return
			}
			assert.Equalf(t, tt.want, got, "excludeHiddenFormFields(%v, %v)", tt.args.formName, tt.args.desc)
		})
	}
}
