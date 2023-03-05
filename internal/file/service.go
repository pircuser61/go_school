package file

import (
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Service struct {
	minio   *minio.Client
	bucket  string
	baseURL string
}

func NewService(cfg *Config) (*Service, error) {
	accessKeyID := os.Getenv(cfg.AccessEnvKey)
	secretAccessKey := os.Getenv(cfg.SecretAccessEnvKey)

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.UseSSl,
	}

	minioClient, err := minio.New(cfg.Addr, opts)
	if err != nil {
		return nil, err
	}

	return &Service{
		minio:   minioClient,
		bucket:  cfg.BucketName,
		baseURL: cfg.BaseURL,
	}, nil
}
