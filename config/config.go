package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/ferio/etcd"
	"github.com/Ferlab-Ste-Justine/ferio/logger"
)

type BinariesCleanupConfig struct {
	Interval        time.Duration
	MaximumBinaries int64         `yaml:"maximum_binaries"`
}

type Config struct {
	Etcd            etcd.EtcdConfig
	BinariesDir     string                `yaml:"binaries_dir"`
	BinariesCleanup BinariesCleanupConfig `yaml:"binaries_cleanup"`
	SystemdService  string                `yaml:"systemd_service"`
	SystemdEnvFile  string                `yaml:"systemd_env_file"`
	Host            string
	MinioApiPort    int64                 `yaml:"minio_api_port"`
	LogLevel        string                `yaml:"log_level"`
}

func getConfigFilePath() string {
	path := os.Getenv("FERIO_CONFIG_FILE")
	if path == "" {
		return "config.yml"
	}
	return path
}

func (c *Config) GetLogLevel() int64 {
	logLevel := strings.ToLower(c.LogLevel)
	switch logLevel {
	case "error":
		return logger.ERROR
	case "warning":
		return logger.WARN
	case "debug":
		return logger.DEBUG
	default:
		return logger.INFO
	}
}

func GetConfig() (Config, error) {
	var c Config

	b, err := ioutil.ReadFile(getConfigFilePath())
	if err != nil {
		return c, errors.New(fmt.Sprintf("Error reading the configuration file: %s", err.Error()))
	}
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return c, errors.New(fmt.Sprintf("Error parsing the configuration file: %s", err.Error()))
	}

	if c.Host == "" {
		hostname, hostnameErr := os.Hostname()
		if hostnameErr != nil {
			return c, errors.New(fmt.Sprintf("Error retrieving hostname: %s", hostnameErr.Error()))
		}
		c.Host = hostname
	}

	if c.BinariesCleanup.MaximumBinaries == 0 {
		c.BinariesCleanup.MaximumBinaries = 3
	}

	if c.BinariesCleanup.Interval == 0 {
		c.BinariesCleanup.Interval = 24 * time.Hour
	}

	return c, nil
}
