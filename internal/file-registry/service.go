package file_registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	fileregistry "gitlab.services.mts.ru/jocasta/file-registry/pkg/proto/gen/file-registry/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	getFileById         = "/api/fileregistry/v1/file/"
	saveFile            = "/api/fileregistry/v1/file/upload"
	dispositionHeader   = "Content-Disposition"
	authorizationHeader = "Authorization"
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
		c:       conn,
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

func (s *Service) GetAttachmentsInfo(ctx context.Context, attachments map[string][]entity.Attachment) (map[string][]FileInfo, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachments_info")
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

func (s *Service) GetAttachments(ctx context.Context, attachments []entity.Attachment) ([]email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachments")
	defer span.End()

	res := make([]email.Attachment, 0, len(attachments))

	for i := range attachments {
		a := attachments[i]
		file, err := s.getAttachment(ctxLocal, a.FileID)
		if err != nil {
			return nil, err
		}
		res = append(res, file)
	}
	return res, nil
}

func (s *Service) SaveFile(ctx context.Context, token string, name string, file []byte) (string, error) {
	ctx, span := trace.StartSpan(ctx, "save_file")
	defer span.End()

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	filePart, err := writer.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}

	_, err = filePart.Write(file)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	reqURL := s.restURL + saveFile
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set(authorizationHeader, token)
	resp, err := s.restCli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got bad status: %s", resp.Status)
	}

	id := fileID{}
	err = json.NewDecoder(resp.Body).Decode(&id)
	if err != nil {
		return "", err
	}

	return id.Data, nil
}
