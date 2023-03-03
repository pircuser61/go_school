package file

import (
	"github.com/minio/minio-go/v7"
)

type Service struct {
	minio                                  *minio.Client
	minioSchemaBucket                      string
	minioStaticBucket                      string
	minioSchemaFilesBucket                 string
	minioTempFilesBucket                   string
	minioBlueprintJustificationsBucketName string
	minioJocastaAttachmentsBucket          string
	baseURL                                string
}
