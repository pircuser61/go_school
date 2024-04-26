package fileregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"

	"github.com/hashicorp/go-retryablehttp"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	fileregistry "gitlab.services.mts.ru/jocasta/file-registry/pkg/proto/gen/file-registry/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
)

const (
	getFileByID         = "/api/fileregistry/v1/file/"
	saveFile            = "/api/fileregistry/v1/file/upload"
	dispositionHeader   = "Content-Disposition"
	authorizationHeader = "Authorization"
)

type Service struct {
	restCli *retryablehttp.Client
	restURL string

	c       *grpc.ClientConn
	grpcCLi fileregistry.FileServiceClient
}

func NewService(cfg Config, log logger.Logger) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	}

	if cfg.MaxRetries != 0 {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(cfg.MaxRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(cfg.RetryDelay)),
			grpc_retry.WithPerRetryTimeout(cfg.Timeout),
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx context.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to fileregistry")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.GRPC, opts...)
	if err != nil {
		return nil, err
	}

	client := fileregistry.NewFileServiceClient(conn)

	return &Service{
		c:       conn,
		restCli: httpclient.NewClient(&http.Client{}, log, cfg.MaxRetries, cfg.RetryDelay),
		restURL: cfg.REST,
		grpcCLi: client,
	}, nil
}

func (s *Service) GetAttachmentLink(ctx context.Context, attachments []AttachInfo) ([]AttachInfo, error) {
	_, span := trace.StartSpan(ctx, "get_attachment_info")
	defer span.End()

	for k, v := range attachments {
		link, err := s.grpcCLi.GetFileLinkById(ctx, &fileregistry.GetFileLinkRequest{
			FileId: v.FileID,
		})
		if err != nil {
			return nil, err
		}

		attachments[k].ExternalLink = link.Url
	}

	return attachments, nil
}

func (s *Service) getAttachmentInfo(ctx context.Context, fileID string) (FileInfo, error) {
	_, span := trace.StartSpan(ctx, "get_attachment_info")
	defer span.End()

	res, err := s.grpcCLi.GetFileInfoById(ctx,
		&fileregistry.GetFileInfoRequest{
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

func (s *Service) getAttachment(ctx context.Context, fileID, workNumber, clientID string) (email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachment")
	defer span.End()

	reqURL := s.restURL + getFileByID + fileID

	req, err := retryablehttp.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return email.Attachment{}, err
	}

	req.Header.Set("Work-Number", workNumber)
	req.Header.Set("Clientid", clientID)

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

func (s *Service) GetAttachments(ctx context.Context,
	attachments []entity.Attachment,
	workNumber, clientID string,
) ([]email.Attachment, error) {
	ctxLocal, span := trace.StartSpan(ctx, "get_attachments")
	defer span.End()

	res := make([]email.Attachment, 0, len(attachments))

	for i := range attachments {
		a := attachments[i]

		file, err := s.getAttachment(ctxLocal, a.FileID, workNumber, clientID)
		if err != nil {
			return nil, err
		}

		res = append(res, file)
	}

	return res, nil
}

func (s *Service) SaveFile(ctx context.Context, token, clientID, name string, file []byte, workNumber string) (string, error) {
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

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, reqURL, buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Work-Number", workNumber)
	req.Header.Set("Clientid", clientID)
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
