package file

type Config struct {
	Addr               string `yaml:"addr"`
	AccessEnvKey       string `yaml:"access_env_key"`
	SecretAccessEnvKey string `yaml:"secret_access_env_key"`
	UseSSl             bool   `yaml:"use_ssl"`
	BucketName         string `yaml:"bucket_name"`
	BaseURL            string `yaml:"base_url"`
}
