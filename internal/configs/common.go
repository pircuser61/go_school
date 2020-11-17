package configs

import (
	"io/ioutil"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type TracingConfig struct {
	URL            string  `yaml:"url"`
	SampleFraction float64 `yaml:"sample_fraction"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	stringDuration := ""

	err := unmarshal(&stringDuration)
	if err != nil {
		return err
	}

	d.Duration, err = time.ParseDuration(stringDuration)

	return err
}

type URL struct {
	*url.URL
}

func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	stringURL := ""

	err := unmarshal(&stringURL)
	if err != nil {
		return err
	}

	u.URL, err = url.Parse(stringURL)

	return err
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
