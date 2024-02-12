package script

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotificationParams_Validate(t *testing.T) {
	tests := []struct {
		name               string
		notificationParams NotificationParams
		wantErr            error
	}{
		{
			name: "Fill people",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Fill emails",
			notificationParams: NotificationParams{
				People:          []string{},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Nil people",
			notificationParams: NotificationParams{
				People:          nil,
				Emails:          []string{},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrNotificationListIsEmpty,
		},
		{
			name: "Nil emails",
			notificationParams: NotificationParams{
				People:          []string{},
				Emails:          nil,
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrNotificationListIsEmpty,
		},
		{
			name: "Nil people and fill other array",
			notificationParams: NotificationParams{
				People:          nil,
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Nil Emails and fill other array",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          nil,
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Fill array and empty string field",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Fill array and set UsersFromSchema",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "A",
				Subject:         "",
				Text:            "",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Fill array and set Subject",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "A",
				Text:            "",
			},
			wantErr: ErrEmptyText,
		},
		{
			name: "Fill array and set Text",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "",
				Subject:         "",
				Text:            "B",
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Fill array and set Text field",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "A",
				Subject:         "B",
				Text:            "C",
			},
			wantErr: nil,
		},
		{
			name: "Fill array",
			notificationParams: NotificationParams{
				People: []string{"A", "B", "C"},
				Emails: []string{"A", "B", "C"},
			},
			wantErr: ErrOneOfSeveralStringFieldsIsEmpty,
		},
		{
			name: "Fill text field",
			notificationParams: NotificationParams{
				Subject:         "A",
				Text:            "B",
				UsersFromSchema: "",
			},
			wantErr: ErrNotificationListIsEmpty,
		},
		{
			name: "Nil array and fill Text field",
			notificationParams: NotificationParams{
				People:          nil,
				Emails:          nil,
				UsersFromSchema: "",
				Subject:         "B",
				Text:            "C",
			},
			wantErr: ErrNotificationListIsEmpty,
		},
		{
			name: "Wrong text source type",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "A",
				Subject:         "B",
				Text:            "C",
				TextSourceType:  "asdfasdf",
			},
			wantErr: ErrUnknownTextSourceType,
		},
		{
			name: "Empty text value with no empty in text source",
			notificationParams: NotificationParams{
				People:          []string{"A", "B", "C"},
				Emails:          []string{"A", "B", "C"},
				UsersFromSchema: "A",
				Subject:         "B",
				Text:            "Hello World!",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.notificationParams.Validate()
			require.ErrorIs(t, err, tt.wantErr, "error not equal")
		})
	}
}
