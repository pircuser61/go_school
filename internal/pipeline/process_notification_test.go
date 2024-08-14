package pipeline

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	om "github.com/iancoleman/orderedmap"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	fileRegestryMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry/mocks"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"
)

func TestBlockRunContext_makeNotificationDescription(t *testing.T) {
	type fields struct {
		TaskID               uuid.UUID
		WorkNumber           string
		ClientID             string
		PipelineID           uuid.UUID
		VersionID            uuid.UUID
		WorkTitle            string
		Initiator            string
		IsTest               bool
		CustomTitle          string
		NotifName            string
		Delegations          human_tasks.Delegations
		VarStore             *store.VariableStore
		UpdateData           *script.BlockUpdateData
		CurrBlockStartTime   time.Time
		skipNotifications    bool
		skipProduce          bool
		Services             RunContextServices
		BlockRunResults      *BlockRunResults
		TaskSubscriptionData TaskSubscriptionData
		OnceProductive       bool
		Productive           bool
	}
	type args struct {
		ctx      context.Context
		nodeName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []om.OrderedMap
		want1   []e.Attachment
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "file by file_id",
			fields: fields{
				skipNotifications: false,
				Productive:        true,
				VarStore:          store.NewStore(),
				Services: RunContextServices{
					FileRegistry: func() fileregistry.Service {
						fileRegistryMock := fileRegestryMocks.NewService(t)

						fileRegistryMock.On("GetAttachmentLink",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attachments []fileregistry.AttachInfo) bool { return true }),
						).Return([]fileregistry.AttachInfo{}, nil)

						fileRegistryMock.On("GetAttachmentsInfo",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attachments map[string][]entity.Attachment) bool { return true }),
						).Return(map[string][]fileregistry.FileInfo{
							"files": {
								{
									FileID:    "1",
									Name:      "small_test.txt",
									CreatedAt: "",
									Size:      123,
								},
							},
						}, nil)

						fileRegistryMock.On("GetAttachments",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attach []entity.Attachment) bool { return true }),
							mock.MatchedBy(func(wNumber string) bool { return true }),
							mock.MatchedBy(func(clientID string) bool { return true }),
						).Return([]e.Attachment{
							{
								Name:    "small_test.txt",
								Content: []byte("hello world"),
								Type:    e.EmbeddedAttachment,
							},
						}, nil)

						return fileRegistryMock
					}(),
					Storage: func() db.Database {
						dbMock := &mocks.MockedDatabase{}

						apBodyJson := `{"field-uuid-1":{"type":"object","properties":{"file_id":"1"}}}`
						apBody := om.New()
						err := apBody.UnmarshalJSON([]byte(apBodyJson))

						if err != nil {
							panic("failed unmarshal application body data in initial application attachment tester")
						}

						dbMock.On("GetTaskRunContext",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return(entity.TaskRunContext{
							InitialApplication: entity.InitialApplication{
								Description:     "",
								ApplicationBody: *apBody,
								Keys: map[string]string{
									"file_id":       "file_id",
									"properties":    "properties",
									"external_link": "external_link",
								},
							},
						}, nil)

						dbMock.On("GetAdditionalDescriptionForms",
							mock.MatchedBy(func(workNumber string) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return([]entity.DescriptionForm{}, nil)

						return dbMock
					}(),
				},
			},
			args: args{
				ctx:      context.Background(),
				nodeName: "",
			},
			want: func() []om.OrderedMap {
				res := make([]om.OrderedMap, 0, 1)
				m := om.New()
				m.SetEscapeHTML(true)
				m.Set("attachExist", true)
				m.Set("attachLinks", []fileregistry.AttachInfo{})
				m.Set("attachList", []e.Attachment{
					{
						Name:    "small_test.txt",
						Content: []byte("hello world"),
						Type:    e.EmbeddedAttachment,
					},
				})
				m.SortKeys(func(keys []string) {
					sort.Slice(keys, func(i, j int) bool {
						return keys[i] < keys[j]
					})
				})
				res = append(res, *m)

				return res
			}(),
			want1: []e.Attachment{
				{
					Name:    "small_test.txt",
					Content: []byte("hello world"),
					Type:    e.EmbeddedAttachment,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "file by external_link",
			fields: fields{
				skipNotifications: false,
				Productive:        true,
				VarStore:          store.NewStore(),
				Services: RunContextServices{
					FileRegistry: func() fileregistry.Service {
						fileRegistryMock := fileRegestryMocks.NewService(t)

						fileRegistryMock.On("GetAttachmentsInfo",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							map[string][]entity.Attachment{
								"files": {},
							},
						).Return(map[string][]fileregistry.FileInfo{}, nil)

						return fileRegistryMock
					}(),
					Storage: func() db.Database {
						dbMock := &mocks.MockedDatabase{}

						apBodyJson := `{"field-uuid-1":{"type":"object","properties":{"external_link":"mts.ru/file/1"}}}`
						apBody := om.New()
						err := apBody.UnmarshalJSON([]byte(apBodyJson))

						if err != nil {
							panic("failed unmarshal application body data in initial application attachment tester")
						}

						dbMock.On("GetTaskRunContext",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return(entity.TaskRunContext{
							InitialApplication: entity.InitialApplication{
								Description:     "",
								ApplicationBody: *apBody,
								Keys: map[string]string{
									"file_id":       "file_id",
									"properties":    "properties",
									"external_link": "external_link",
								},
							},
						}, nil)

						dbMock.On("GetAdditionalDescriptionForms",
							mock.MatchedBy(func(workNumber string) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return([]entity.DescriptionForm{}, nil)

						return dbMock
					}(),
				},
			},
			args: args{
				ctx:      context.Background(),
				nodeName: "",
			},
			want: func() []om.OrderedMap {
				res := make([]om.OrderedMap, 0, 1)
				m := om.New()
				m.SetEscapeHTML(true)
				m.Set("external_link (external_link)", "mts.ru/file/1")

				res = append(res, *m)

				return res
			}(),
			want1:   []e.Attachment{},
			wantErr: assert.NoError,
		},
		{
			// приоритетным считается external_link
			name: "file by file_id and external_link",
			fields: fields{
				skipNotifications: false,
				Productive:        true,
				VarStore:          store.NewStore(),
				Services: RunContextServices{
					FileRegistry: func() fileregistry.Service {
						fileRegistryMock := fileRegestryMocks.NewService(t)

						fileRegistryMock.On("GetAttachmentsInfo",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attachments map[string][]entity.Attachment) bool { return true }),
						).Return(map[string][]fileregistry.FileInfo{
							"files": {
								{},
							},
						}, nil)

						return fileRegistryMock
					}(),
					Storage: func() db.Database {
						dbMock := &mocks.MockedDatabase{}

						apBodyJson := `{"field-uuid-1":{"type":"object","properties":{"file_id":"1","external_link":"mts.ru/file/2"}}}`
						apBody := om.New()
						err := apBody.UnmarshalJSON([]byte(apBodyJson))

						if err != nil {
							panic("failed unmarshal application body data in initial application attachment tester")
						}

						dbMock.On("GetTaskRunContext",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return(entity.TaskRunContext{
							InitialApplication: entity.InitialApplication{
								Description:     "",
								ApplicationBody: *apBody,
								Keys: map[string]string{
									"file_id":       "file_id",
									"properties":    "properties",
									"external_link": "external_link",
								},
							},
						}, nil)

						dbMock.On("GetAdditionalDescriptionForms",
							mock.MatchedBy(func(workNumber string) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return([]entity.DescriptionForm{}, nil)

						return dbMock
					}(),
				},
			},
			args: args{
				ctx:      context.Background(),
				nodeName: "",
			},
			want: func() []om.OrderedMap {
				res := make([]om.OrderedMap, 0, 1)
				m := om.New()
				m.SetEscapeHTML(true)
				m.Set("external_link (external_link)", "mts.ru/file/2")

				res = append(res, *m)

				return res
			}(),
			want1:   []e.Attachment{},
			wantErr: assert.NoError,
		},
		{
			name: "file by external_link with hidden field",
			fields: fields{
				skipNotifications: false,
				Productive:        true,
				VarStore:          store.NewStore(),
				Services: RunContextServices{
					FileRegistry: func() fileregistry.Service {
						fileRegistryMock := fileRegestryMocks.NewService(t)

						fileRegistryMock.On("GetAttachmentsInfo",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attachments map[string][]entity.Attachment) bool { return true }),
						).Return(map[string][]fileregistry.FileInfo{
							"files": {
								{},
							},
						}, nil)

						return fileRegistryMock
					}(),
					Storage: func() db.Database {
						dbMock := &mocks.MockedDatabase{}

						apBodyJson := `{"field-uuid-1":{"type":"object","properties":{"external_link":"mts.ru/file/3","hidden_foo":"bar","unhidden_foo":"biz"}}}`
						apBody := om.New()
						err := apBody.UnmarshalJSON([]byte(apBodyJson))

						if err != nil {
							panic("failed unmarshal application body data in initial application attachment tester")
						}

						dbMock.On("GetTaskRunContext",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return(entity.TaskRunContext{
							InitialApplication: entity.InitialApplication{
								Description:     "",
								ApplicationBody: *apBody,
								Keys: map[string]string{
									"file_id":       "file_id",
									"properties":    "properties",
									"external_link": "external_link",
									"hidden_foo":    "hidden_foo",
									"unhidden_foo":  "unhidden_foo",
								},
								HiddenFields: []string{"properties (hidden_foo)"},
							},
						}, nil)

						dbMock.On("GetAdditionalDescriptionForms",
							mock.MatchedBy(func(workNumber string) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return([]entity.DescriptionForm{}, nil)

						return dbMock
					}(),
				},
			},
			args: args{
				ctx:      context.Background(),
				nodeName: "",
			},
			want: func() []om.OrderedMap {
				res := make([]om.OrderedMap, 0, 1)
				m := om.New()
				m.SetEscapeHTML(true)
				m.Set("external_link (external_link)", "mts.ru/file/3")
				m.Set("unhidden_foo (unhidden_foo)", "biz")

				res = append(res, *m)

				return res
			}(),
			want1:   []e.Attachment{},
			wantErr: assert.NoError,
		},
		{
			name: "big files",
			fields: fields{
				skipNotifications: false,
				Productive:        true,
				VarStore:          store.NewStore(),
				Services: RunContextServices{
					FileRegistry: func() fileregistry.Service {
						fileRegistryMock := fileRegestryMocks.NewService(t)

						fileRegistryMock.On("GetAttachmentLink",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attachments []fileregistry.AttachInfo) bool { return true }),
						).Return([]fileregistry.AttachInfo{
							{
								FileID:       "3",
								Name:         "big_test3.txt",
								CreatedAt:    "",
								Size:         15728640,
								ExternalLink: "mts.ru/file/3",
							},
						}, nil, nil)

						fileRegistryMock.On("GetAttachmentsInfo",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attachments map[string][]entity.Attachment) bool { return true }),
						).Return(map[string][]fileregistry.FileInfo{
							"files": {
								{
									FileID:    "2",
									Name:      "big_test2.txt",
									CreatedAt: "",
									Size:      15728640,
								},
								{
									FileID:    "3",
									Name:      "big_test3.txt",
									CreatedAt: "",
									Size:      15728640,
								},
							},
						}, nil, nil)

						fileRegistryMock.On("GetAttachments",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(attach []entity.Attachment) bool { return true }),
							mock.MatchedBy(func(wNumber string) bool { return true }),
							mock.MatchedBy(func(clientID string) bool { return true }),
						).Return([]e.Attachment{
							{
								Name:    "big_test2.txt",
								Content: []byte("hello world"),
								Type:    e.EmbeddedAttachment,
							},
						}, nil, nil)

						return fileRegistryMock
					}(),
					Storage: func() db.Database {
						dbMock := &mocks.MockedDatabase{}

						apBodyJson := `{"field-uuid-1":{"type":"object","properties":{"file_id":"2"}},
										"field-uuid-2":{"type":"object","properties":{"file_id":"3"}}}`
						apBody := om.New()
						err := apBody.UnmarshalJSON([]byte(apBodyJson))

						if err != nil {
							panic("failed unmarshal application body data in initial application attachment tester")
						}

						dbMock.On("GetTaskRunContext",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return(entity.TaskRunContext{
							InitialApplication: entity.InitialApplication{
								Description:     "",
								ApplicationBody: *apBody,
								Keys: map[string]string{
									"file_id":       "file_id",
									"properties":    "properties",
									"external_link": "external_link",
								},
							},
						}, nil)

						dbMock.On("GetAdditionalDescriptionForms",
							mock.MatchedBy(func(workNumber string) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
						).Return([]entity.DescriptionForm{}, nil)

						return dbMock
					}(),
				},
			},
			args: args{
				ctx:      context.Background(),
				nodeName: "",
			},
			want: func() []om.OrderedMap {
				res := make([]om.OrderedMap, 0, 1)
				m := om.New()
				m.SetEscapeHTML(true)
				m.Set("attachExist", true)
				m.Set("attachLinks", []fileregistry.AttachInfo{
					{
						FileID:       "3",
						Name:         "big_test3.txt",
						CreatedAt:    "",
						Size:         15728640,
						ExternalLink: "mts.ru/file/3",
					},
				})
				m.Set("attachList", []e.Attachment{
					{
						Name:    "big_test2.txt",
						Content: []byte("hello world"),
						Type:    e.EmbeddedAttachment,
					},
				})
				m.SortKeys(func(keys []string) {
					sort.Slice(keys, func(i, j int) bool {
						return keys[i] < keys[j]
					})
				})
				res = append(res, *m)

				return res
			}(),
			want1: []e.Attachment{
				{
					Name:    "big_test2.txt",
					Content: []byte("hello world"),
					Type:    e.EmbeddedAttachment,
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCtx := &BlockRunContext{
				TaskID:               tt.fields.TaskID,
				WorkNumber:           tt.fields.WorkNumber,
				ClientID:             tt.fields.ClientID,
				PipelineID:           tt.fields.PipelineID,
				VersionID:            tt.fields.VersionID,
				WorkTitle:            tt.fields.WorkTitle,
				Initiator:            tt.fields.Initiator,
				IsTest:               tt.fields.IsTest,
				CustomTitle:          tt.fields.CustomTitle,
				NotifName:            tt.fields.NotifName,
				Delegations:          tt.fields.Delegations,
				VarStore:             tt.fields.VarStore,
				UpdateData:           tt.fields.UpdateData,
				CurrBlockStartTime:   tt.fields.CurrBlockStartTime,
				skipNotifications:    tt.fields.skipNotifications,
				skipProduce:          tt.fields.skipProduce,
				Services:             tt.fields.Services,
				BlockRunResults:      tt.fields.BlockRunResults,
				TaskSubscriptionData: tt.fields.TaskSubscriptionData,
				OnceProductive:       tt.fields.OnceProductive,
				Productive:           tt.fields.Productive,
			}
			got, got1, err := runCtx.makeNotificationDescription(tt.args.ctx, tt.args.nodeName, false)
			if !tt.wantErr(t, err, fmt.Sprintf("makeNotificationDescription(%v, %v)", tt.args.ctx, tt.args.nodeName)) {
				return
			}
			got[0].SortKeys(func(keys []string) {
				sort.Slice(keys, func(i, j int) bool {
					return keys[i] < keys[j]
				})
			})
			assert.Equalf(t, tt.want, got, "makeNotificationDescription(%v, %v)", tt.args.ctx, tt.args.nodeName)
			assert.Equalf(t, tt.want1, got1, "makeNotificationDescription(%v, %v)", tt.args.ctx, tt.args.nodeName)
		})
	}
}
