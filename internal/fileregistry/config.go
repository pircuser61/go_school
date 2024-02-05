package fileregistry

type Config struct {
	REST string `yaml:"rest"`
	GRPC string `yaml:"grpc"`
}
