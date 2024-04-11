package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func Test_createGoNotificationBlock(t *testing.T) {
	const (
		example         = "example"
		title           = "title"
		shortTitle      = "Нода Email"
		loginFromSlice0 = "pilzner1"
		loginFromSlice1 = "users1"
		loginFromSlice2 = "userss1"
		loginFromSlice3 = "usersss1"
		loginFromSlice4 = "userssss1"

		emails          = "test@mts.ru"
		people          = "user"
		usersFromSchema = "sd_app_0.application_body.usersFromSchema;form_0.usersFromSchema;form_1.usersFromSchema"
		text            = "test"
		subject         = "users"
	)

	myStorage := makeStorage()
	varStore := store.NewStore()

	varStore.SetValue("sd_app_0.application_body.usersFromSchema", []interface{}{
		map[string]interface{}{
			"username": loginFromSlice0,
		},
		map[string]interface{}{
			"username": loginFromSlice1,
		},
		map[string]interface{}{
			"username": loginFromSlice2,
		},
	})

	varStore.SetValue("form_0.usersFromSchema", map[string]interface{}{
		"username": loginFromSlice3,
		"fullname": "test test test",
	})
	varStore.SetValue("form_1.usersFromSchema", map[string]interface{}{
		"username": loginFromSlice4,
		"fullname": "test2 test test",
	})

	next := []entity.Socket{
		{
			ID:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			ID:           rejectedSocketID,
			Title:        script.RejectSocketTitle,
			NextBlockIds: []string{"next_1"},
		},
	}

	type args struct {
		name   string
		ef     *entity.EriusFunc
		runCtx *BlockRunContext
	}

	tests := []struct {
		name    string
		args    args
		want    *GoNotificationBlock
		wantErr bool
	}{
		{
			name: "can not get notification parameters",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params:     nil,
					Sockets:    next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid notification parameters",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params:     []byte("{}"),
					Sockets:    next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty fields in params",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationTitle,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.NotificationParams{
							Emails:          []string{},
							People:          []string{},
							UsersFromSchema: "",
							Text:            "",
							Subject:         "",
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty string fields in params",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationTitle,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.NotificationParams{
							Emails:          []string{emails},
							People:          []string{people},
							UsersFromSchema: "",
							Text:            "",
							Subject:         "",
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty array fields in params",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationTitle,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.NotificationParams{
							Emails:          nil,
							People:          nil,
							UsersFromSchema: usersFromSchema,
							Text:            text,
							Subject:         subject,
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty UsersFromSchema fields in params",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationTitle,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.NotificationParams{
							Emails:          []string{},
							People:          []string{loginFromSlice0},
							UsersFromSchema: "",
							Text:            text,
							Subject:         subject,
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want: &GoNotificationBlock{
				Name:           example,
				ShortName:      shortTitle,
				Title:          title,
				Input:          map[string]string{},
				Output:         map[string]string{},
				happenedEvents: make([]entity.NodeEvent, 0),
				State: &NotificationData{
					People:          []string{loginFromSlice0},
					Emails:          []string{},
					Text:            text,
					Subject:         subject,
					UsersFromSchema: map[string]struct{}{},
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: false,
		},
		{
			name: "acceptance test",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          varStore,
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoNotificationID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.NotificationParams{
							Emails:          []string{emails},
							People:          []string{people},
							UsersFromSchema: usersFromSchema,
							Text:            text,
							Subject:         subject,
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want: &GoNotificationBlock{
				Name:           example,
				ShortName:      shortTitle,
				Title:          title,
				Input:          map[string]string{},
				Output:         map[string]string{},
				happenedEvents: make([]entity.NodeEvent, 0),
				State: &NotificationData{
					People:  []string{people},
					Emails:  []string{emails},
					Text:    text,
					Subject: subject,
					UsersFromSchema: map[string]struct{}{
						loginFromSlice0: {},
						loginFromSlice1: {},
						loginFromSlice2: {},
						loginFromSlice3: {},
						loginFromSlice4: {},
					},
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, _, err := createGoNotificationBlock(ctx, tt.args.name, tt.args.ef, tt.args.runCtx, nil)
			if got != nil {
				got.RunContext = nil
			}

			assert.Equalf(t, tt.wantErr, err != nil, "createGoNotificationBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
			assert.Equalf(t, tt.want, got, "createGoNotificationBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
		})
	}
}

func Test_GoNotificationBlock_notificationBlockText(t *testing.T) {
	varStore := store.NewStore()

	varStore.SetValue(
		"start_0.initiator",
		map[string]string{
			"email": "davoro26@mts.ru",
		},
	)

	varStore.SetValue("start_0.initiator.email", "davoro26@mts.ru")

	tests := []struct {
		name                     string
		block                    *GoNotificationBlock
		expectedNotificationText string
		expectedErr              error
	}{
		{
			name: "default text field source notification text",
			block: &GoNotificationBlock{
				State: &NotificationData{
					Text: "aboba",
				},
			},

			expectedNotificationText: "aboba",
		},

		{
			name: "default text field source notification text",
			block: &GoNotificationBlock{
				State: &NotificationData{
					Text:           "aboba",
					TextSourceType: script.TextFieldSource,
				},
			},

			expectedNotificationText: "aboba",
		},

		{
			name: "context value source, context value not exists",
			block: &GoNotificationBlock{
				RunContext: &BlockRunContext{
					VarStore: varStore,
				},
				State: &NotificationData{
					Text:           "undefinedpath",
					TextSourceType: script.VarContextSource,
				},
			},
			expectedNotificationText: "",
		},

		{
			name: "context value source, context value not string",
			block: &GoNotificationBlock{
				RunContext: &BlockRunContext{
					VarStore: varStore,
				},
				State: &NotificationData{
					Text:           "start_0.initiator",
					TextSourceType: script.VarContextSource,
				},
			},
			expectedNotificationText: "",
		},

		{
			name: "context value source, context value exists and string type",
			block: &GoNotificationBlock{
				RunContext: &BlockRunContext{
					VarStore: varStore,
				},
				State: &NotificationData{
					Text:           "start_0.initiator.email",
					TextSourceType: script.VarContextSource,
				},
			},
			expectedNotificationText: "davoro26@mts.ru",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := tt.block.notificationBlockText()

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedNotificationText, text)
		})
	}
}
