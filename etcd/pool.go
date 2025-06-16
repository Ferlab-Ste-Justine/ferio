package etcd

import (
	"errors"
	"fmt"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/ferio/pool"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

const ETCD_POOLS_CONFIG_KEY = "%spools"

type MinioServerPools struct {
	Version string
	Pools   pool.MinioServerPools
}

func GetMinioServerPools(cli *client.EtcdClient, prefix string) (*MinioServerPools, int64, error) {
	var pools MinioServerPools
	
	info, err := cli.GetKey(fmt.Sprintf(ETCD_POOLS_CONFIG_KEY, prefix), client.GetKeyOptions{})
	if err != nil {
		return nil, -1, err
	}

	if !info.Found() {
		return nil, -1, errors.New("Minio server pools configuration is not set")
	}

	err = yaml.Unmarshal([]byte(info.Value), &pools)
	if err != nil {
		return nil, -1, errors.New(fmt.Sprintf("Error parsing the server pools configuration: %s", err.Error()))
	}

	return &pools, info.ModRevision, nil
}

const ETCD_POOLS_TASKS_ACKNOWLEDGMENT_KEY = "%stasks/pools/%s/acknowledgment/"
const ETCD_POOLS_TASKS_MINIO_SHUTDOWN_KEY = "%stasks/pools/%s/minio_shutdown/"
const ETCD_POOLS_TASKS_SYSTEMD_UPDATE_KEY = "%stasks/pools/%s/systemd_update/"

func (pools *MinioServerPools) getTaskKeys(prefix string) (string, string, string) {
	return fmt.Sprintf(ETCD_POOLS_TASKS_ACKNOWLEDGMENT_KEY, prefix, pools.Version),
	fmt.Sprintf(ETCD_POOLS_TASKS_MINIO_SHUTDOWN_KEY, prefix, pools.Version),
	fmt.Sprintf(ETCD_POOLS_TASKS_SYSTEMD_UPDATE_KEY, prefix, pools.Version)
}

type PoolsUpdate struct {
	AcknowledgmentDone bool
	MinioShutdownDone  bool
	SystemdUpdateDone  bool
	CurrentTaskStatus  *Task
}

func (upd *PoolsUpdate) GetTaskKey(prefix string, pools *MinioServerPools) string  {
	if !upd.AcknowledgmentDone {
		return fmt.Sprintf(ETCD_POOLS_TASKS_ACKNOWLEDGMENT_KEY, prefix, pools.Version)
	} else if !upd.MinioShutdownDone {
		return fmt.Sprintf(ETCD_POOLS_TASKS_MINIO_SHUTDOWN_KEY, prefix, pools.Version)
	}

	return fmt.Sprintf(ETCD_POOLS_TASKS_SYSTEMD_UPDATE_KEY, prefix, pools.Version)
}

func (upd *PoolsUpdate) IsDone() bool {
	return upd.AcknowledgmentDone && upd.MinioShutdownDone && upd.SystemdUpdateDone
}

func (pools *MinioServerPools) GetUpdate(cli *client.EtcdClient, prefix string) (*PoolsUpdate, error) {
	ackKey, shutdownKey, systemdKey := pools.getTaskKeys(prefix)
	
	for _, key := range []string{ackKey, shutdownKey, systemdKey} {
		tk, _, err := GetTask(cli, key)
		if err != nil {
			return nil, err
		}

		if !tk.CanContinue(pools.Pools.CountHosts()) {
			return &PoolsUpdate{
				AcknowledgmentDone: key != ackKey,
				MinioShutdownDone: key != ackKey && key != shutdownKey,
				SystemdUpdateDone: false,
				CurrentTaskStatus: tk,
			}, nil
		}
	}

	return &PoolsUpdate{
		AcknowledgmentDone: true,
		MinioShutdownDone: true,
		SystemdUpdateDone: true,
		CurrentTaskStatus: nil,
	}, nil
}

func (upd *PoolsUpdate) HandleNextTask(cli *client.EtcdClient, prefix string, pools *MinioServerPools, host string, action TaskAction) error {
	tkKey := upd.GetTaskKey(prefix, pools)
	
	if upd.CurrentTaskStatus.HasToDo(host) {
		err := action()
		if err != nil {
			return err
		}

		err = MarkTaskDoneBySelf(cli, tkKey, host)
		if err != nil {
			return err
		}
	}

	err := WaitOnTaskCompletion(cli, tkKey, pools.Pools.CountHosts())
	if err != nil {
		return err
	}

	_, shutdownKey, systemdKey := pools.getTaskKeys(prefix)
	if !upd.AcknowledgmentDone {
		upd.AcknowledgmentDone = true
		tk, _, err := GetTask(cli, shutdownKey)
		if err != nil {
			return err
		}
		upd.CurrentTaskStatus = tk
	} else if !upd.MinioShutdownDone {
		upd.MinioShutdownDone = true
		tk, _, err := GetTask(cli, systemdKey)
		if err != nil {
			return err
		}
		upd.CurrentTaskStatus = tk
	} else {
		upd.SystemdUpdateDone = true
		upd.CurrentTaskStatus = nil
	}

	return nil
}