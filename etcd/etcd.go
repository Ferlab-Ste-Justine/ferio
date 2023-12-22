package etcd

import (
	"context"
	"time"

	"github.com/Ferlab-Ste-Justine/ferio/auth"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

const ETCD_BINARIES_CONFIG_PREFIX = "%sbinaries/"

type EtcdConfig struct {
	ConfigPrefix      string        `yaml:"config_prefix"`
	WorkspacePrefix   string        `yaml:"workspace_prefix"`
	Endpoints         []string
	ConnectionTimeout time.Duration	`yaml:"connection_timeout"`
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	RetryInterval     time.Duration `yaml:"retry_interval"`
	Retries           uint64
	Auth              auth.Auth
}

func GetClient(conf EtcdConfig) (*client.EtcdClient, error)  {
	passErr := conf.Auth.ResolvePassword()
	if passErr != nil {
		return nil, passErr
	}

	return client.Connect(context.Background(), client.EtcdClientOptions{
		ClientCertPath:    conf.Auth.ClientCert,
		ClientKeyPath:     conf.Auth.ClientKey,
		CaCertPath:        conf.Auth.CaCert,
		Username:          conf.Auth.Username,
		Password:          conf.Auth.Password,
		EtcdEndpoints:     conf.Endpoints,
		ConnectionTimeout: conf.ConnectionTimeout,
		RequestTimeout:    conf.RequestTimeout,
		RetryInterval:     conf.RetryInterval,
		Retries:           conf.Retries,
	})
}