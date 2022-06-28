package mail

type Config struct {
	Broker   string `yaml:"broker"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Queue    string `yaml:"queue"`
	Database string `yaml:"database"`
	From     struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"from"`
}
