package file

import (
	c "context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/minio/minio-go/v7"

	"go.opencensus.io/trace"
)

func (s *Service) SaveFile(ctx c.Context, ext, origName string, file io.Reader, size int64) (id string, err error) {
	ctxLocal, span := trace.StartSpan(ctx, "saveFile")
	defer span.End()

	log := logger.GetLogger(ctx)
	log.Info("sending file to storage origName: ", origName)
	log.Info("sending file to storage size: ", size)

	opts := minio.PutObjectOptions{}
	if origName != "" {
		opts.UserMetadata = map[string]string{"Filename": origName}
	}

	id, name, err := s.GenerateUniqFileName(ctxLocal, ext, s.bucket)
	if err != nil {
		return id, err
	}

	log.Info("sending file to storage: ", name)
	_, err = s.minio.PutObject(ctxLocal, s.bucket, id, file, size, opts)
	if err != nil {
		return id, err
	}

	return id, nil
}

func (s *Service) GenerateUniqFileName(ctx c.Context, ext, bucket string) (id, name string, err error) {
	id = uuid.New().String()

	_, err = s.minio.StatObject(ctx, bucket, fmt.Sprintf("%s.%s", id, ext), minio.GetObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return id, fmt.Sprintf("%s.%s", id, ext), nil
		}

		return "", "", err
	}

	return s.GenerateUniqFileName(ctx, ext, bucket)
}
