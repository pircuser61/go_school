package file_registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	fileregistry "gitlab.services.mts.ru/jocasta/file-registry/pkg/proto/gen/file-registry/v1"
)

const (
	getFileById       = "/api/fileregistry/v1/file/"
	dispositionHeader = "Content-Disposition"
)

type Service struct {
	restCli *http.Client
	restURL string

	c       *grpc.ClientConn
	grpcCLi fileregistry.FileServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
	conn, err := grpc.Dial(cfg.GRPC, opts...)
	if err != nil {
		return nil, err
	}
	client := fileregistry.NewFileServiceClient(conn)

	return &Service{
		restCli: &http.Client{},
		restURL: cfg.REST,
		grpcCLi: client,
	}, nil
}

func (s *Service) getAttachmentInfo(ctx context.Context, fileId string) (FileInfo, error) {
	_, span := trace.StartSpan(ctx, "get_attachment_info")
	defer span.End()

	res, err := s.grpcCLi.GetFileInfoById(ctx,
		&fileregistry.GetFileInfoRequest{
			FileId: fileId,
		},
	)

	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		FileId:    res.FileId,
		Name:      res.Name,
		CreatedAt: res.CreatedAt,
		Size:      res.Size,
	}, nil
}

func (s *Service) GetAttachmentsInfo(ctx context.Context, attachments map[string][]string) (map[string][]FileInfo, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachments_info")
	defer span.End()

	res := make(map[string][]FileInfo)

	for k := range attachments {
		aa := attachments[k]
		filesInfo := make([]FileInfo, 0, len(aa))
		for _, a := range aa {
			fileInfo, err := s.getAttachmentInfo(ctxLocal, a)
			if err != nil {
				return nil, err
			}
			filesInfo = append(filesInfo, fileInfo)
		}
		res[k] = filesInfo
	}
	return res, nil
}

func (s *Service) getAttachment(ctx context.Context, fileId string) (email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachment")
	defer span.End()

	reqURL := s.restURL + getFileById + fileId

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return email.Attachment{}, err
	}

	resp, err := s.restCli.Do(req)
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

func (s *Service) GetAttachments(ctx context.Context, attachments []string) ([]email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachments")
	defer span.End()

	res := make([]email.Attachment, 0, len(attachments))

	for i := range attachments {
		a := attachments[i]
		file, err := s.getAttachment(ctxLocal, a)
		if err != nil {
			return nil, err
		}
		res = append(res, file)
	}
	return res, nil
}
