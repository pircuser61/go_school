package utils

import "testing"

func TestMakeTaskTitle(t *testing.T) {
	type args struct {
		versionTitle string
		customTitle  string
		isTest       bool
	}

	tests := []struct {
		name    string
		args    args
		wantRes string
	}{
		{
			name: "is test work",
			args: args{
				versionTitle: "version name",
				customTitle:  "",
				isTest:       true,
			},
			wantRes: "version name (ТЕСТОВАЯ ЗАЯВКА)",
		},
		{
			name: "is not test work",
			args: args{
				versionTitle: "version name",
				customTitle:  "",
				isTest:       false,
			},
			wantRes: "version name",
		},
		{
			name: "is test work with custom title",
			args: args{
				versionTitle: "version name",
				customTitle:  "custom title",
				isTest:       true,
			},
			wantRes: "custom title (ТЕСТОВАЯ ЗАЯВКА)",
		},
		{
			name: "is not test work with custom title",
			args: args{
				versionTitle: "version name",
				customTitle:  "custom title",
				isTest:       false,
			},
			wantRes: "custom title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := MakeTaskTitle(tt.args.versionTitle, tt.args.customTitle, tt.args.isTest); gotRes != tt.wantRes {
				t.Errorf("MakeTaskTitle() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
