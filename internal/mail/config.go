package mail

import "time"

type Config struct {
	Broker     string `yaml:"broker"`
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Queue      string `yaml:"queue"`
	Database   string `yaml:"database"`
	ImagesPath string `yaml:"images_path"`
	From       struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"from"`
	SdAddress    string `yaml:"sd_address"`
	FetchEmail   string
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}
