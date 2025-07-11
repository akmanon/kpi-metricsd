package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Cfg struct {
	Server ServerConfig `yaml:"server"`
	LogCfg LogCfg       `yaml:"log_config"`
	KPIs   []KPI        `yaml:"kpis"`
}

type ServerConfig struct {
	Port        int    `yaml:"port"`
	MetricsPath string `yaml:"metrics_path"`
}

type LogCfg struct {
	SourceLogFile    string `yaml:"source_log_file"`
	RedirectLogFile  string `yaml:"redirect_log_file"`
	RotatedLogFile   string `yaml:"rotated_log_file"`
	RotationInterval string `yaml:"rotation_interval"`
}

type KPI struct {
	Name         string            `yaml:"name"`
	Regex        string            `yaml:"regex"`
	CustomLabels map[string]string `yaml:"custom_labels"`
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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil

}

func (c *Cfg) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.LogCfg.RedirectLogFile == "" {
		return fmt.Errorf("redirect_log_file is not defined in config")
	}
	if c.LogCfg.SourceLogFile == "" {
		return fmt.Errorf("source_log_file is not defined in config")
	}
	if c.LogCfg.RotatedLogFile == "" {
		return fmt.Errorf("rotated_log_file is not defined in config")
	}
	if c.LogCfg.RotationInterval == "" {
		return fmt.Errorf("rotation_interval is not defined in config")
	}
	if len(c.KPIs) == 0 {
		return fmt.Errorf("no KPIs defined in config")
	}
	for _, kpi := range c.KPIs {
		if kpi.Name == "" || kpi.Regex == "" {
			return fmt.Errorf("KPI name or regex is not defined in config")
		}
	}
	return nil
}
