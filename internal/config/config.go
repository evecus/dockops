package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTPPort  int    `yaml:"http_port"`
	HTTPSPort int    `yaml:"https_port"`
	CertPath  string `yaml:"cert_path"`
	KeyPath   string `yaml:"key_path"`
	DataPath  string `yaml:"data_path"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		HTTPPort:  8080,
		HTTPSPort: 8443,
		DataPath:  "./data",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cfg.DataPath, 0755); err != nil {
		return nil, err
	}

	return cfg, nil
}
