package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Cfg struct {
	LogCfg LogCfg `yaml:"log_config"`
}

type LogCfg struct {
	SourceLogFile    string `yaml:"source_log_file"`
	RedirectLogFile  string `yaml:"redirect_log_file"`
	RotatedLogFile   string `yaml:"rotated_log_file"`
	RotationInterval string `yaml:"rotation_interval"`
}

func LoadCfg(cfgPath string) (*Cfg, error) {

	cfgFile, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %w", err)
	}
	var cfg Cfg

	if err := yaml.Unmarshal(cfgFile, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %w", err)
	}

	if _, err := time.ParseDuration(cfg.LogCfg.RotationInterval); err != nil {
		return nil, fmt.Errorf("failed to parse rotation_interval in config file %w", err)
	}

	return &cfg, nil

}
