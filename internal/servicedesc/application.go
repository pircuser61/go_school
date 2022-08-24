package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"github.com/iancoleman/orderedmap"
)

const (
	getFileByID        = "/api/herald/v1/file/%s"
	getApplicationBody = "/api/herald/v1/application/%s/body"

	dispositionHeader = "Content-Disposition"
)

func (s *Service) getAttachment(ctx context.Context, id string) (email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachment")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s", s.sdURL, fmt.Sprintf(getFileByID, id))

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return email.Attachment{}, err
	}

	resp, err := s.cli.Do(req)
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

type applicationBody struct {
	Body             orderedmap.OrderedMap `json:"body"`
	AttachmentFields []string              `json:"attachment_fields"`
	Keys             map[string]string     `json:"keys"`
}

func (s *Service) GetSchemaFieldsByApplication(ctx context.Context, applicationID string) (*applicationBody, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_schema_fields_by_application")
	defer span.End()

	reqURL := fmt.Sprintf("%s%s", s.sdURL, fmt.Sprintf(getApplicationBody, applicationID))

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}

	res := &applicationBody{}
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res, nil
}
