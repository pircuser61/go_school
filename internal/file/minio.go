package file

import (
	"context"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"go.opencensus.io/trace"
)

// nolint:gocritic // it's more comfortable to work with config as a value
func connectToMinio(c FileStorage) (*minio.Client, error) {
	accessKeyID := os.Getenv(c.AccessEnvKey)
	secretAccessKey := os.Getenv(c.SecretAccessEnvKey)

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: c.UseSSl,
	}

	return minio.New(c.Addr, opts)
}

func (ss *Service) SaveFile(ctx context.Context, name, originalName, bucket string, file io.Reader, size int64) error {
	ctxLocal, span := trace.StartSpan(ctx, "saveFile")
	defer span.End()

	opts := minio.PutObjectOptions{}
	if originalName != "" {
		opts.UserMetadata = map[string]string{"Filename": originalName}
	}
	_, err := ss.minio.PutObject(ctxLocal, bucket, name, file, size, opts)
	if err != nil {
		return err
	}
	return nil
}

func (ss *Service) copyFileToBucket(ctx context.Context, fromBucket, toBucket, id string) error {
	ctxLocal, span := trace.StartSpan(ctx, "copyFileToBucket")
	defer span.End()

	src := minio.CopySrcOptions{
		Bucket: fromBucket,
		Object: id,
	}
	dst := minio.CopyDestOptions{
		Bucket: toBucket,
		Object: id,
	}

	_, err := ss.minio.CopyObject(ctxLocal, dst, src)
	return err
}

type File struct {
	Id   string
	Name string
	Size int
}
