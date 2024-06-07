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
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	getFileByID       = "/api/fileregistry/v1/file/"
	saveFile          = "/api/fileregistry/v1/file/upload"
	dispositionHeader = "Content-Disposition"
)

func (s *service) GetAttachmentLink(ctx c.Context, attachments []AttachInfo) ([]AttachInfo, error) {
	_, span := trace.StartSpan(ctx, "file_registry.get_attachment_link")
	defer span.End()

	log := logger.GetLogger(ctx)

	for k, v := range attachments {
		count := 0
		ctx = c.WithValue(ctx, retryCnt{}, &count)
		link, err := s.grpcCLi.GetFileLinkById(ctx, &fr.GetFileLinkRequest{
			FileId: v.FileID,
		})
		attempt, ok := ctx.Value(retryCnt{}).(*int)

		if err != nil {
			log.WithField("traceID", span.SpanContext().TraceID.String()).
				Warning("Pipeliner failed to connect to fileregistry.  Exceeded max retry count: ", *attempt)

			return nil, err
		}

		if ok && *attempt > 0 {
			log.WithField("traceID", span.SpanContext().TraceID.String()).
				Warning("Pipeliner successfully reconnected to fileregistry: ", attempt)
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

	count := 0
	log := logger.GetLogger(ctx)
	url := s.restURL + getFileByID + fileID
	ctxLocal = c.WithValue(ctxLocal, retryCnt{}, &count)

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, url, http.NoBody)
	if err != nil {
		return em.Attachment{}, err
	}

	req.Header.Set("Work-Number", workNumber)
	req.Header.Set("Clientid", clientID)

	resp, err := s.restCli.Do(req)
	attempt, ok := req.Context().Value(retryCnt{}).(*int)

	if err != nil {
		log.WithField("traceID", span.SpanContext().TraceID.String()).
			Warning("Pipeliner failed to connect to fileregistry.  Exceeded max retry count: ", *attempt)

		return em.Attachment{}, err
	}
	defer resp.Body.Close()

	if ok && *attempt > 0 {
		log.WithField("traceID", span.SpanContext().TraceID.String()).
			Warning("Pipeliner successfully reconnected to fileregistry: ", *attempt)
	}

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
