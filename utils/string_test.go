package utils

import (
	"reflect"
	"testing"
)

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

func TestGetAttachmentsIds(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Success",
			args: args{text: `"application_body":{
            "recipient":{
               "email":"test@mts.ru",
               "phone":889876646",
               "position":"разработчик",
               "fullOrgUnit":"ООО \"МТС бизнес-процессов&Центр Платформа автоматизации"
            },
            "field-uuid-2":"доработка 1+",
            "field-uuid-3":"{"\file_id"\:"\277fb00d-045d-48ad-b220-89ef5a488053"\,"\name"\:"\test"\}"
            "field-uuid-4":"[{"\file_id"\:"\95033b61-4d6f-4c8b-be75-7af4806c930d"\,"\name"\:"\test"\}]"
            "field-uuid-5":"{"\external_link"\:"\ad03c9bb-d417-465c-b763-9eed2a505576"\,"\name"\:"\test"\}"
            "field-uuid-6":"{"\external_link"\:"\2dd6ee01-75aa-4e70-8a94-afe6b21608b0"\,"\name"\:"\test"\}"
            "field-uuid-7":"{"\attachment"\:"\4ed2bdcc-8829-4eaa-bb19-6e8c0b3e8be0"\,"\name"\:"\test"\}"
            "field-uuid-8":"[{"\name"\:"\test_1"\"\attachment"\:"\b16b6bce-d209-4999-b42c-f98761e3d945"\},
							 {"\name"\:"\test_2"\"\attachment"\:"\1ebd906f-a8b6-4091-8817-70a8f25fed9e"\}]"`,
			},
			want: []string{
				"277fb00d-045d-48ad-b220-89ef5a488053",
				"95033b61-4d6f-4c8b-be75-7af4806c930d",
				"ad03c9bb-d417-465c-b763-9eed2a505576",
				"2dd6ee01-75aa-4e70-8a94-afe6b21608b0",
				"4ed2bdcc-8829-4eaa-bb19-6e8c0b3e8be0",
				"b16b6bce-d209-4999-b42c-f98761e3d945",
				"1ebd906f-a8b6-4091-8817-70a8f25fed9e",
			},
		},
		{
			name: "Invalid file uuid",
			args: args{text: `"application_body":{
            "recipient":{
               "email":"test@mts.ru",
               "phone":889876646",
               "position":"разработчик",
               "fullOrgUnit":"ООО \"МТС бизнес-процессов&Центр Платформа автоматизации"
            },
            "field-uuid-2":"доработка 1+",
            "field-uuid-3":"{"\file_id"\:"\1277fb00d-045d-48ad-b220-89ef5a488053"\,"\name"\:"\test"\}"
            "field-uuid-4":"[{"\file_id"\:"\95033b61-4d6f-4c8b-be75-7af4806c930d"\,"\name"\:"\test"\}]"`,
			},
			want: []string{"95033b61-4d6f-4c8b-be75-7af4806c930d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAttachmentsIds(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAttachmentsIds() = %v, want %v", got, tt.want)
			}
		})
	}
}
