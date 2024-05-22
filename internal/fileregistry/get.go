package fileregistry

import (
	c "context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/hashicorp/go-retryablehttp"

	"go.opencensus.io/trace"

	fr "gitlab.services.mts.ru/jocasta/file-registry/pkg/proto/gen/file-registry/v1"

	em "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	getFileByID       = "/api/fileregistry/v1/file/"
	saveFile          = "/api/fileregistry/v1/file/upload"
	dispositionHeader = "Content-Disposition"
)

func (s *service) GetAttachmentLink(ctx c.Context, attachments []AttachInfo) ([]AttachInfo, error) {
	_, span := trace.StartSpan(ctx, "file_registry.get_attachment_info")
	defer span.End()

	for k, v := range attachments {
		link, err := s.grpcCLi.GetFileLinkById(ctx, &fr.GetFileLinkRequest{
			FileId: v.FileID,
		})
		if err != nil {
			return nil, err
		}

		attachments[k].ExternalLink = link.Url
	}

	return attachments, nil
}

func (s *service) getAttachmentInfo(ctx c.Context, fileID string) (FileInfo, error) {
	_, span := trace.StartSpan(ctx, "file_registry.get_attachment_info")
	defer span.End()

	res, err := s.grpcCLi.GetFileInfoById(ctx,
		&fr.GetFileInfoRequest{
			FileId: fileID,
		},
	)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		FileID:    res.FileId,
		Name:      res.Name,
		CreatedAt: res.CreatedAt,
		Size:      res.Size,
	}, nil
}

func (s *service) GetAttachmentsInfo(ctx c.Context, attachments map[string][]entity.Attachment) (map[string][]FileInfo, error) {
	ctxLocal, span := trace.StartSpan(ctx, "file_registry.get_attachments_info")
	defer span.End()

	res := make(map[string][]FileInfo)

	for k := range attachments {
		aa := attachments[k]
		filesInfo := make([]FileInfo, 0, len(aa))

		for _, a := range aa {
			fileInfo, err := s.getAttachmentInfo(ctxLocal, a.FileID)
			if err != nil {
				return nil, err
			}

			filesInfo = append(filesInfo, fileInfo)
		}

		res[k] = filesInfo
	}

	return res, nil
}

func (s *service) getAttachment(ctx c.Context, fileID, workNumber, clientID string) (em.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "file_registry.get_attachment")
	defer span.End()

	url := s.restURL + getFileByID + fileID

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, url, http.NoBody)
	if err != nil {
		return em.Attachment{}, err
	}

	req.Header.Set("Work-Number", workNumber)
	req.Header.Set("Clientid", clientID)

	resp, err := s.restCli.Do(req)
	if err != nil {
		return em.Attachment{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return em.Attachment{}, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return em.Attachment{}, err
	}

	// temp decision
	name := regexp.MustCompile(`^attachment; filename=`).ReplaceAllString(resp.Header.Get(dispositionHeader), "")

	return em.Attachment{
		Name:    name,
		Content: data,
		Type:    em.EmbeddedAttachment,
	}, nil
}

func (s *service) GetAttachments(ctx c.Context, attach []entity.Attachment, wNumber, clientID string) ([]em.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "file_registry.get_attachments")
	defer span.End()

	res := make([]em.Attachment, 0, len(attach))

	for i := range attach {
		a := attach[i]

		file, err := s.getAttachment(ctxLocal, a.FileID, wNumber, clientID)
		if err != nil {
			return nil, err
		}

		res = append(res, file)
	}

	return res, nil
}
