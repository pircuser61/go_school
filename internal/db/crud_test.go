package db

import (
	"reflect"
	"testing"
)

func Test_mergeValues(t *testing.T) {
	type args struct {
		stepsValues []map[string]interface{}
	}

	const (
		workID    = "9612e3d1-20fc-4890-9c0c-122057604ef1"
		testLogin = "testLogin"
	)

	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "success",
			args: args{stepsValues: []map[string]interface{}{
				0: {
					stepNameVariable:    "start_0",
					"start_0.work_id":   workID,
					"execution_0.login": testLogin,
				},
				1: {
					stepNameVariable:       "execution_0",
					"start_0.work_id":      workID,
					"execution_0.login":    testLogin,
					"execution_0.decision": "executed",
					"approver_0.sla":       9,
				},
				2: {
					stepNameVariable:       "approver_0",
					"start_0.work_id":      workID,
					"execution_0.login":    testLogin,
					"execution_0.decision": "executed",
					"approver_0.sla":       10,
				},
			}},
			want: map[string]interface{}{
				"start_0.work_id":      workID,
				"execution_0.login":    "testLogin",
				"execution_0.decision": "executed",
				"approver_0.sla":       10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeValues(tt.args.stepsValues); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeValues() = %v, want %v", got, tt.want)
			}
		})
	}
}
