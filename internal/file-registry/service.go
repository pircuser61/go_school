package file_registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
)

const (
	getFileById       = "/api/fileregistry/v1/file/"
	dispositionHeader = "Content-Disposition"
)

type Service struct {
	Cli *http.Client
	URL string
}

func NewService(cfg Config) (*Service, error) {
	return &Service{
		Cli: &http.Client{},
		URL: cfg.URL,
	}, nil
}

func (s *Service) getAttachment(ctx context.Context, fileId string) (email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachment")
	defer span.End()

	reqURL := s.URL + getFileById + fileId

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return email.Attachment{}, err
	}

	resp, err := s.Cli.Do(req)
	if err != nil {
		return email.Attachment{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return email.Attachment{}, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return email.Attachment{}, err
	}

	// temp decision
	name := regexp.MustCompile(`^attachment; filename=`).ReplaceAllString(resp.Header.Get(dispositionHeader), "")
	return email.Attachment{
		Name:    name,
		Content: data,
		Type:    email.EmbeddedAttachment,
	}, nil
}

func (s *Service) GetAttachments(ctx context.Context, attachments map[string][]string) (map[string][]email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachments")
	defer span.End()

	res := make(map[string][]email.Attachment)

	for k := range attachments {
		aa := attachments[k]
		files := make([]email.Attachment, 0, len(aa))
		for _, a := range aa {
			file, err := s.getAttachment(ctxLocal, a)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
		res[k] = files
	}
	return res, nil
}
