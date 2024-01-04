package etcd

import (
	"errors"
	"fmt"
	"strings"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

const ETCD_POOLS_CONFIG_KEY = "%spools"

type MinioServerPool struct {
	ApiPort           int64  `yaml:"api_port"`
	DomainTemplate    string `yaml:"domain_template"`
	ServerCountBegin  int64  `yaml:"server_count_begin"`
	ServerCountEnd    int64  `yaml:"server_count_end"`
	MountPathTemplate string `yaml:"mount_path_template"`
	MountCount        int64  `yaml:"mount_count"`
}

func (pool *MinioServerPool) Stringify() string {
	urls := fmt.Sprintf(
		"https://%s:%d",
		fmt.Sprintf(
			pool.DomainTemplate,
			fmt.Sprintf("{%d...%d}", pool.ServerCountBegin, pool.ServerCountEnd),
		),
		pool.ApiPort,
	)
	mounts := fmt.Sprintf(
		pool.MountPathTemplate,
		fmt.Sprintf("{1...%d}", pool.MountCount),
	)

	return fmt.Sprintf("%s%s", urls, mounts)
}

type MinioServerPools struct {
	Version string
	Pools   []MinioServerPool
}

func (pools *MinioServerPools) CountHosts() int64 {
	count := int64(0)
	for _, pool := range (*pools).Pools {
		count += (pool.ServerCountEnd - pool.ServerCountBegin + 1)
	}
	return count
}

func (pools *MinioServerPools) Stringify() string {
	stringifiedPools := []string{}
	for _, pool := range (*pools).Pools {
		stringifiedPools = append(stringifiedPools, pool.Stringify())
	}

	return strings.Join(stringifiedPools, " ")
}

func GetMinioServerPools(cli *client.EtcdClient, prefix string) (*MinioServerPools, error) {
	var pools MinioServerPools
	
	info, err := cli.GetKey(fmt.Sprintf(ETCD_POOLS_CONFIG_KEY, prefix), client.GetKeyOptions{})
	if err != nil {
		return nil, err
	}

	if !info.Found() {
		return nil, errors.New("Minio server pools configuration is not set")
	}

	err = yaml.Unmarshal([]byte(info.Value), &pools)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error parsing the server pools configuration: %s", err.Error()))
	}

	return &pools, nil
}

const ETCD_POOLS_TASKS_ACKNOWLEDGMENT_KEY = "%s/tasks/pools/%s/acknowledgment"
const ETCD_POOLS_TASKS_MINIO_SHUTDOWN_KEY = "%s/tasks/pools/%s/minio_shutdown"
const ETCD_POOLS_TASKS_SYSTEMD_UPDATE_KEY = "%s/tasks/pools/%s/systemd_update"

func (pools *MinioServerPools) getTaskKeys(prefix string) (string, string, string) {
	return fmt.Sprintf(ETCD_POOLS_TASKS_ACKNOWLEDGMENT_KEY, prefix, pools.Version),
	fmt.Sprintf(ETCD_POOLS_TASKS_MINIO_SHUTDOWN_KEY, prefix, pools.Version),
	fmt.Sprintf(ETCD_POOLS_TASKS_SYSTEMD_UPDATE_KEY, prefix, pools.Version)
}

type PoolsUpdateStatus struct {
	AcknowledgmentDone bool
	MinioShutdownDone  bool
	SystemdUpdateDone  bool
	CurrentTaskStatus  *Task
}

func (status *PoolsUpdateStatus) IsDone() bool {
	return status.AcknowledgmentDone && status.MinioShutdownDone && status.SystemdUpdateDone
}

func (pools *MinioServerPools) GetUpdateStatus(cli *client.EtcdClient, prefix string) (*PoolsUpdateStatus, error) {
	ackKey, shutdownKey, systemdKey := pools.getTaskKeys(prefix)
	
	for _, key := range []string{ackKey, shutdownKey, systemdKey} {
		tk, _, err := GetTaskStatus(cli, key)
		if err != nil {
			return nil, err
		}

		if !tk.CanContinue(pools) {
			return &PoolsUpdateStatus{
				AcknowledgmentDone: key != ackKey,
				MinioShutdownDone: key != ackKey && key != shutdownKey,
				SystemdUpdateDone: false,
				CurrentTaskStatus: tk,
			}, nil
		}
	}

	return &PoolsUpdateStatus{
		AcknowledgmentDone: true,
		MinioShutdownDone: true,
		SystemdUpdateDone: true,
		CurrentTaskStatus: nil,
	}, nil
}

func (pools *MinioServerPools) HandleNextTask(cli *client.EtcdClient, prefix string, status *PoolsUpdateStatus, host string, action TaskAction) error {	
	if status.CurrentTaskStatus.HasToDo(host) {
		err := action()
		if err != nil {
			return err
		}

		err = MarkTaskDoneBySelf(cli, prefix, host)
		if err != nil {
			return err
		}
	}

	err := WaitOnTaskCompletion(cli, prefix, pools.CountHosts())
	if err != nil {
		return err
	}

	ackKey, shutdownKey, systemdKey := pools.getTaskKeys(prefix)
	if !status.AcknowledgmentDone {
		status.AcknowledgmentDone = true
		tk, _, err := GetTaskStatus(cli, ackKey)
		if err != nil {
			return err
		}
		status.CurrentTaskStatus = tk
	} else if !status.MinioShutdownDone {
		status.MinioShutdownDone = true
		tk, _, err := GetTaskStatus(cli, shutdownKey)
		if err != nil {
			return err
		}
		status.CurrentTaskStatus = tk
	} else {
		status.SystemdUpdateDone = true
		tk, _, err := GetTaskStatus(cli, systemdKey)
		if err != nil {
			return err
		}
		status.CurrentTaskStatus = tk
	}

	return nil
}