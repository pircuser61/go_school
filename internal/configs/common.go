package configs

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"
)

type TracingConfig struct {
	URL            string  `yaml:"url"`
	SampleFraction float64 `yaml:"sample_fraction"`
}

func Read(path string, cfg interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Errorf("cant read config file: %s", err.Error())
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return errors.Errorf("cant parse config: %s", err.Error())
	}

	return nil
}
