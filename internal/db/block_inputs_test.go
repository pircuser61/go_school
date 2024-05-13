package db

import (
	"encoding/json"
	"reflect"
	"testing"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func Test_trySetNewParams(t *testing.T) {
	type args struct {
		stepParams json.RawMessage
		inputs     entity.BlockInputs
	}

	stepParams := []byte(`{"sla":1,"approver":"ivan","otherParam":true}`)
	var stepParamsEmpty []byte

	inputs := entity.BlockInputs{
		{
			Name:  "sla",
			Value: 2,
		},
		{
			Name:  "approver",
			Value: "gogen",
		},
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				stepParams: stepParams,
				inputs:     inputs,
			},
			want:    map[string]interface{} {
				"sla": float64(2),
				"approver": "gogen",
				"otherParam": true,
			},
			wantErr: false,
		},
		{
			name: "empty step params",
			args: args{
				stepParams: stepParamsEmpty,
				inputs:     inputs,
			},
			want:    map[string]interface{} {},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := trySetNewParams(tt.args.stepParams, tt.args.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("trySetNewParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			res := map[string]interface{}{}
			_ = json.Unmarshal(got, &res)

			if !reflect.DeepEqual(res, tt.want) {
				t.Errorf("trySetNewParams() \n got = %+v \n want =%+v", res, tt.want)
			}
		})
	}
}
