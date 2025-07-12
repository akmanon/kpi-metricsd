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
	Port        int         `yaml:"port"`
	MetricsPath string      `yaml:"metrics_path"`
	PushGateway PushGateway `yaml:"pushgateway"`
}

type PushGateway struct {
	Enabled  bool   `yaml:"enabled"`
	URL      string `yaml:"url"`
	Job      string `yaml:"job"`
	Instance string `yaml:"instance"`
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil

}

func (c *Cfg) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	if len(c.KPIs) == 0 {
		return fmt.Errorf("no KPIs defined in config")
	}

	if err := c.validateLogCfg(); err != nil {
		return err
	}
	if err := c.validateKPICfg(); err != nil {
		return err
	}
	if err := c.validateServerCfg(); err != nil {
		return err
	}

	return nil
}

func (c *Cfg) validateServerCfg() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server invalid port number")
	}
	if c.Server.MetricsPath == "" {
		return fmt.Errorf("server metric path is not defined in config")
	}
	if c.Server.PushGateway.Enabled {
		if c.Server.PushGateway.URL == "" {
			return fmt.Errorf("pushgateway url is not defined")
		}
		if c.Server.PushGateway.Job == "" {
			return fmt.Errorf("job url is not defined")
		}
		if c.Server.PushGateway.Instance == "" {
			return fmt.Errorf("instance url is not defined")
		}
	}

	return nil
}

func (c *Cfg) validateLogCfg() error {
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
	rotationInterval, err := time.ParseDuration(c.LogCfg.RotationInterval)
	if err != nil {
		return fmt.Errorf("failed to parse rotation_interval in config file %w", err)
	}
	if rotationInterval < 60*time.Second {
		return fmt.Errorf("rotation interval should be > 60 seconds ")
	}
	return nil
}

func (c *Cfg) validateKPICfg() error {
	for _, kpi := range c.KPIs {
		if kpi.Name == "" || kpi.Regex == "" {
			return fmt.Errorf("KPI name or regex is not defined in config")
		}
	}
	return nil
}
