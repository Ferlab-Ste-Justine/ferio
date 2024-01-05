package etcd

import (
	"errors"
	"fmt"
	"strings"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/ferio/logger"

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

const ETCD_POOLS_TASKS_ACKNOWLEDGMENT_KEY = "%s/tasks/pools/%s/acknowledgment"
const ETCD_POOLS_TASKS_MINIO_SHUTDOWN_KEY = "%s/tasks/pools/%s/minio_shutdown"
const ETCD_POOLS_TASKS_SYSTEMD_UPDATE_KEY = "%s/tasks/pools/%s/systemd_update"

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

		if !tk.CanContinue(pools) {
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
	if upd.CurrentTaskStatus.HasToDo(host) {
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
	if !upd.AcknowledgmentDone {
		upd.AcknowledgmentDone = true
		tk, _, err := GetTask(cli, ackKey)
		if err != nil {
			return err
		}
		upd.CurrentTaskStatus = tk
	} else if !upd.MinioShutdownDone {
		upd.MinioShutdownDone = true
		tk, _, err := GetTask(cli, shutdownKey)
		if err != nil {
			return err
		}
		upd.CurrentTaskStatus = tk
	} else {
		upd.SystemdUpdateDone = true
		tk, _, err := GetTask(cli, systemdKey)
		if err != nil {
			return err
		}
		upd.CurrentTaskStatus = tk
	}

	return nil
}

type ServerPoolsChangeAction func(*MinioServerPools, *MinioRelease) error

func HandleServerPoolsChanges(cli *client.EtcdClient, prefix string, startPools *MinioServerPools, action ServerPoolsChangeAction, log logger.Logger) <-chan error {
	errCh := make(chan error)
	go func() {
		log.Infof("[etcd] Starting to watch for server pool changes")

		configKey := fmt.Sprintf(ETCD_POOLS_CONFIG_KEY, prefix)

		pools, rev, getErr := GetMinioServerPools(cli, prefix)
		if getErr != nil {
			errCh <- getErr
			close(errCh)
			return
		}

		if (*pools).Version != (*startPools).Version {
			log.Infof("[etcd] Handling new server pools configuration at version %s", (*pools).Version)

			rel, _, getErr := GetMinioRelease(cli, prefix)
			if getErr != nil {
				errCh <- getErr
				close(errCh)
				return
			}

			actErr := action(pools, rel)
			if actErr != nil {
				errCh <- actErr
				close(errCh)
				return
			}
		}

		wcCh := cli.Watch(configKey, client.WatchOptions{
			Revision: rev + 1,
			IsPrefix: false,
			TrimPrefix: false,
		})

		for info := range wcCh {
			pools := MinioServerPools{}

			if info.Error != nil {
				errCh <- info.Error
				close(errCh)
				return
			}

			if len(info.Changes.Deletions) > 0 {
				errCh <- errors.New("Got an unexpected etcd key deletion while looking for server pools changes")
				close(errCh)
				return
			}

			err := yaml.Unmarshal([]byte(info.Changes.Upserts[configKey].Value), &pools)
			if err != nil {
				errCh <- errors.New(fmt.Sprintf("Error parsing the server pools configuration: %s", err.Error()))
				close(errCh)
				return
			}

			log.Infof("[etcd] Handling new server pools configuration at version %s", pools.Version)

			rel, _, getErr := GetMinioRelease(cli, prefix)
			if getErr != nil {
				errCh <- getErr
				close(errCh)
				return
			}

			actErr := action(&pools, rel)
			if actErr != nil {
				errCh <- actErr
				close(errCh)
				return
			}
		}
	}()
	return errCh
}