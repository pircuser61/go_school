package db

import (
	"reflect"
	"testing"
)

func Test_mergeStates(t *testing.T) {
	type args struct {
		in []map[string]map[string]interface{}
	}

	tests := []struct {
		name    string
		args    args
		wantRes map[string]map[string]interface{}
	}{
		{
			name: "success",
			args: args{in: []map[string]map[string]interface{}{
				0: {"approver_0": map[string]interface{}{"sla": 10}},
				1: {"approver_0": map[string]interface{}{"sla": 11}},
				2: {"approver_1": map[string]interface{}{"sla": 12}},
			}},
			wantRes: map[string]map[string]interface{}{
				"approver_0": {"sla": 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := mergeStates(tt.args.in, []string{"approver_0"}); !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("mergeStates() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
