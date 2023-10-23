package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

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

		emails          = "user@mts.ru"
		people          = "user"
		usersFromSchema = "a.var1;b.var2;var3"
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
			"userName": "noname",
		},
	})

	varStore.SetValue("form_0.usersFromSchema", map[string]interface{}{
		"username": "test",
		"fullname": "test test test",
	})
	varStore.SetValue("form_1.usersFromSchema", map[string]interface{}{
		"username": "test2",
		"fullname": "test2 test test",
	})

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			Id:           rejectedSocketID,
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
					Params:     nil,
					Sockets:    next,
				},
			},
			wantErr: true,
			want:    nil,
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
					Params:     []byte("{}"),
					Sockets:    next,
				},
			},
			wantErr: true,
			want:    nil,
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
			wantErr: true,
			want:    nil,
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
			wantErr: true,
			want:    nil,
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
			wantErr: true,
			want:    nil,
		},
		{
			name: "acceptance test",
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
			want:    nil,
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
