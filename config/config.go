package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/ferio/etcd"
	"github.com/Ferlab-Ste-Justine/ferio/logger"
	"github.com/Ferlab-Ste-Justine/ferio/systemd"
)

type Config struct {
	Etcd            etcd.EtcdConfig
	BinariesDir     string                 `yaml:"binaries_dir"`
	Host            string
	LogLevel        string                 `yaml:"log_level"`
	MinioServices   []systemd.MinioService `yaml:"minio_services"`
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

	if len(c.MinioServices) == 0 {
		c.MinioServices = []systemd.MinioService{
			systemd.MinioService{
				Name: "minio.service",
				EnvPath: "/etc/minio/env",
			},
		}
	}

	return c, nil
}
