package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func setupTestEnv(t *testing.T, timeouts time.Duration, retryInt time.Duration, retries uint64) *client.EtcdClient {
	cli, err := client.Connect(context.Background(), client.EtcdClientOptions{
		ClientCertPath:    "../test/certs/root.pem",
		ClientKeyPath:     "../test/certs/root.key",
		CaCertPath:        "../test/certs/ca.crt",
		EtcdEndpoints:     []string{"127.0.0.1:3379", "127.0.0.2:3379", "127.0.0.3:3379"},
		ConnectionTimeout: timeouts,
		RequestTimeout:    timeouts,
		RetryInterval:     retryInt,
		Retries:           retries,
	})

	if err != nil {
		t.Errorf("Test setup failed at the connection stage: %s", err.Error())
	}

	user := client.EtcdUser{
		Username: "root",
		Password: "",
		Roles: []string{"root"},
	}

	err = cli.UpsertUser(user)
	if err != nil {
		t.Errorf("Test setup failed at the root user creation stage: %s", err.Error())
	}

	err = cli.SetAuthStatus(true)
	if err != nil {
		t.Errorf("Test setup failed at the auth enabling stage: %s", err.Error())
	}

	return cli
}

func isStringInSlice(val string, slice []string) bool {
	for _, elem := range slice {
		if elem == val {
			return true
		}
	}
	return false
}
