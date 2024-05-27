package api

import (
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"reflect"
	"testing"
)

func Test_getPrevStepState(t *testing.T) {
	type args struct {
		prevContent map[string]interface{}
		stepName    string
	}
	tests := []struct {
		name string
		args args
		want entity.BlockState
	}{
		{
			name: "success",
			args: args{
				prevContent: map[string]interface{}{
					"Errors": "",
					"Steps":  []string{"start_0", "executable_function_0"},
					"State": map[string]interface{}{
						"executable_function_0": map[string]interface{}{
							"version": "v1.0.0-alpha.7",
						},
						"start_0": map[string]interface{}{
							"test": 1,
						},
					},
				},
				stepName: "executable_function_0",
			},
			want: []entity.BlockStateValue{
				{
					Name:  "version",
					Value: "v1.0.0-alpha.7",
				},
			},
		},
		{
			name: "step is not found",
			args: args{
				prevContent: map[string]interface{}{
					"Errors": "",
					"Steps":  []string{"start_0", "executable_function_0"},
					"State": map[string]interface{}{
						"executable_function_1": map[string]interface{}{
							"async": false,
						},
						"start_0": map[string]interface{}{
							"test": 1,
						},
					},
				},
				stepName: "executable_function_0",
			},
			want: []entity.BlockStateValue{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPrevStepState(tt.args.prevContent, tt.args.stepName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPrevStepState() = %v, want %v", got, tt.want)
			}
		})
	}
}
