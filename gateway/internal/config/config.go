package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	CorePath          string `json:"core_path"`
	InstancesPath     string `json:"instances_path"`
	DBPath            string `json:"db_path"`
	AbsenDBPath       string `json:"absen_db_path"`
	RootDeviceIniPath string `json:"root_device_ini_path"`
	GatewayPort       int    `json:"gateway_port"`
	FServiceStartPort int    `json:"fservice_start_port"`
	WatchdogInterval  string `json:"watchdog_interval"`
}

func Default() *Config {
	wd, _ := os.Getwd()
	return &Config{
		CorePath:          wd + "\\core",
		InstancesPath:     wd + "\\instances",
		DBPath:            wd + "\\easylink.db",
		AbsenDBPath:       wd + "\\absen.db",
		RootDeviceIniPath: wd + "\\Device.ini",
		GatewayPort:       7100,
		FServiceStartPort: 7110,
		WatchdogInterval:  "10s",
	}
}

func Load() (*Config, error) {
	cfg := Default()

	cfgPath := os.Getenv("EASYLINK_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.json"
	}
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config.json: %w", err)
		}
	}

	if v := os.Getenv("EASYLINK_CORE_PATH"); v != "" {
		cfg.CorePath = v
	}
	if v := os.Getenv("EASYLINK_INSTANCES_PATH"); v != "" {
		cfg.InstancesPath = v
	}
	if v := os.Getenv("EASYLINK_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("EASYLINK_ABSEN_DB_PATH"); v != "" {
		cfg.AbsenDBPath = v
	}
	if v := os.Getenv("EASYLINK_ROOT_DEVICE_INI_PATH"); v != "" {
		cfg.RootDeviceIniPath = v
	}
	if v := os.Getenv("EASYLINK_GATEWAY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.GatewayPort = p
		}
	}
	if v := os.Getenv("EASYLINK_FSERVICE_START_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.FServiceStartPort = p
		}
	}
	if v := os.Getenv("EASYLINK_WATCHDOG_INTERVAL"); v != "" {
		cfg.WatchdogInterval = v
	}

	return cfg, nil
}

func (c *Config) WatchdogDuration() time.Duration {
	d, err := time.ParseDuration(c.WatchdogInterval)
	if err != nil {
		return 10 * time.Second
	}
	return d
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf(":%d", c.GatewayPort)
}
