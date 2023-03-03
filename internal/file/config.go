package file

type FileStorage struct {
	Addr                         string `yaml:"addr"`
	AccessEnvKey                 string `yaml:"access_env_key"`
	SecretAccessEnvKey           string `yaml:"secret_access_env_key"`
	UseSSl                       bool   `yaml:"use_ssl"`
	SchemasBucketName            string `yaml:"schemas_bucket_name"`
	StaticBucketName             string `yaml:"static_bucket_name"`
	SchemasFilesBucketName       string `yaml:"schema_files_bucket_name"`
	TempFilesBucketName          string `yaml:"temp_files_bucket_name"`
	JocastaAttachmentsBucketName string `yaml:"jocasta_attachments_bucket_name"`
	BaseURL                      string `yaml:"base_url"`
}
