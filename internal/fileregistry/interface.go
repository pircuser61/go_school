package fileregistry

import (
	c "context"

	em "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type Service interface {
	GetAttachmentLink(ctx c.Context, attachments []AttachInfo) ([]AttachInfo, error)
	GetAttachmentsInfo(ctx c.Context, attachments map[string][]entity.Attachment) (map[string][]FileInfo, error)
	GetAttachments(ctx c.Context, attach []entity.Attachment, wNumber, clientID string) ([]em.Attachment, error)
	SaveFile(ctx c.Context, token, clientID, name string, file []byte, workNumber string) (string, error)

	Ping(ctx c.Context) error
}
