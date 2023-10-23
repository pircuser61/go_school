package script

import (
	"testing"
)

func TestNotificationParams_Validate(t *testing.T) {
	type fields struct {
		People          []string
		Emails          []string
		UsersFromSchema string
		Subject         string
		Text            string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Fill people",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          []string{},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Fill emails",
			fields: fields{
				People:          []string{},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Nil people",
			fields: fields{
				People:          nil,
				Emails:          []string{},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Nil emails",
			fields: fields{
				People:          []string{},
				Emails:          nil,
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Nil people and fill other array",
			fields: fields{
				People:          nil,
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Nil Emails and fill other array",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          nil,
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Fill array and empty string field",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Fill array and set UsersFromSchema",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "A",
				Subject:         "",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Fill array and set Subject",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "A",
				Text:            "",
			},
			wantErr: true,
		},
		{
			name: "Fill array and set Text",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "B",
			},
			wantErr: true,
		},
		{
			name: "Fill array and set Text field",
			fields: fields{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "A",
				Subject:         "B",
				Text:            "C",
			},
			wantErr: false,
		},
		{
			name: "Fill array",
			fields: fields{
				People: []string{"A", "B", "C"},
				Emails: []string{"A", "B", "C"},
			},
			wantErr: true,
		},
		{
			name: "Fill text field",
			fields: fields{
				Subject:         "A",
				Text:            "B",
				UsersFromSchema: "C",
			},
			wantErr: true,
		},
		{
			name: "Nil array and fill Text field",
			fields: fields{
				People:          nil,
				Emails:          nil,
				UsersFromSchema: "A",
				Subject:         "B",
				Text:            "C",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &NotificationParams{
				Emails:          tt.fields.Emails,
				People:          tt.fields.People,
				UsersFromSchema: tt.fields.UsersFromSchema,
				Subject:         tt.fields.Subject,
				Text:            tt.fields.Text,
			}
			if err := a.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("%v ValidateSchemas()", err)
			}
		})
	}
}
