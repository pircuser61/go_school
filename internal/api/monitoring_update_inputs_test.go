package api

import "testing"

func Test_validateInputs(t *testing.T) {
	type args struct {
		stepName string
		inputs   map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success",
			args: args{inputs: map[string]interface{}{
				"people":          []string{"vasha"},
				"emails":          []string{},
				"usersFromSchema": "",
				"subject":         "test sbj",
				"text":            "text example",
				"textSourceType":  "",
			},
				stepName: "notification_0"},
			wantErr: false,
		},
		{
			name: "invalid approver",
			args: args{inputs: map[string]interface{}{},
				stepName: "approver_0"},
			wantErr: true,
		},
		{
			name: "unknown step type",
			args: args{inputs: map[string]interface{}{
				"people":          []string{"vasha"},
				"emails":          []string{},
				"usersFromSchema": "",
				"subject":         "test sbj",
				"text":            "text example",
				"textSourceType":  "",
			},
				stepName: "_approver_0"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateInputs(tt.args.stepName, tt.args.inputs); (err != nil) != tt.wantErr {
				t.Errorf("validateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
