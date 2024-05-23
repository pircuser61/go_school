package db

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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

func Test_getType(t *testing.T) {
	tests := []struct {
		name  string
		types *[]string
		items []interface{}
		wants *[]string
	}{
		{
			name:  "clear items",
			items: nil,
			types: nil,
			wants: nil,
		},
		{
			name:  "one field",
			items: []interface{}{"a"},
			types: &[]string{},
			wants: &[]string{"string"},
		},
		{
			name:  "two field",
			items: []interface{}{"a", map[string]interface{}{"a": "b"}},
			types: &[]string{},
			wants: &[]string{"string", "object"},
		},
		{
			name:  "three field",
			items: []interface{}{"a", map[string]interface{}{"a": "b"}, 4},
			types: &[]string{},
			wants: &[]string{"string", "object", "number"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getType(tt.types, tt.items)

			assert.Equal(t, tt.wants, tt.types)
		})
	}
}

func Test_processMap(t *testing.T) {
	tests := []struct {
		name  string
		data  interface{}
		items *[]interface{}
		wants *[]interface{}
	}{
		{
			name:  "clear items",
			items: &[]interface{}{},
			data:  nil,
			wants: &[]interface{}{nil},
		},
		{
			name:  "one string array",
			items: &[]interface{}{},
			data:  []string{"a", "b"},
			wants: &[]interface{}{[]string{"a", "b"}},
		},
		{
			name:  "string array and map",
			items: &[]interface{}{[]string{"a", "b"}},
			data:  map[string]interface{}{"a": "b"},
			wants: &[]interface{}{[]string{"a", "b"}, map[string]interface{}{"a": "b"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processMap(tt.data, tt.items)

			assert.Equal(t, tt.wants, tt.items)
		})
	}
}
