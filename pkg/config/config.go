package config

import (
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

type ExecuteConfig struct {
	HostsFileName  string        `yaml:"hosts_file"`
	AuthMode       string        `yaml:"auth_mode"`
	ExecuteTimeout time.Duration `yaml:"timeout"`
}

type Config struct {
	ExecuteConfig `yaml:"execute_config"`
}

// NewConfig parse config file to config instance.
func NewConfig(cfgFile string) (*Config, error) {
	content, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = yaml.UnmarshalStrict(content, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
