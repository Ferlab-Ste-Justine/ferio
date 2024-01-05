package etcd

import (
	"errors"
	"fmt"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/ferio/logger"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

const ETCD_RELEASE_CONFIG_KEY = "%srelease"

type MinioRelease struct {
	Version  string
	Url      string
	Checksum string
}

func GetMinioRelease(cli *client.EtcdClient, prefix string) (*MinioRelease, int64, error) {
	var rel MinioRelease
	
	info, err := cli.GetKey(fmt.Sprintf(ETCD_RELEASE_CONFIG_KEY, prefix), client.GetKeyOptions{})
	if err != nil {
		return nil, -1, err
	}

	if !info.Found() {
		return nil, -1, errors.New("Minio release configuration is not set")
	}

	err = yaml.Unmarshal([]byte(info.Value), &rel)
	if err != nil {
		return nil, -1, errors.New(fmt.Sprintf("Error parsing the minio release configuration: %s", err.Error()))
	}

	return &rel, info.ModRevision, nil
}

const ETCD_RELEASE_TASKS_BINARY_DOWNLOAD_KEY = "%s/tasks/binary/%s/binary_download"
const ETCD_RELEASE_TASKS_MINIO_SHUTDOWN_KEY = "%s/tasks/binary/%s/minio_shutdown"
const ETCD_RELEASE_TASKS_SYSTEMD_UPDATE_KEY = "%s/tasks/binary/%s/systemd_update"

func (rel *MinioRelease) getTaskKeys(prefix string) (string, string, string) {
	return fmt.Sprintf(ETCD_RELEASE_TASKS_BINARY_DOWNLOAD_KEY, prefix, rel.Version),
	fmt.Sprintf(ETCD_RELEASE_TASKS_MINIO_SHUTDOWN_KEY, prefix, rel.Version),
	fmt.Sprintf(ETCD_RELEASE_TASKS_SYSTEMD_UPDATE_KEY, prefix, rel.Version)
}

type ReleaseUpdate struct {
	DownloadDone       bool
	MinioShutdownDone  bool
	SystemdUpdateDone  bool
	CurrentTaskStatus  *Task
}

func (upd *ReleaseUpdate) IsDone() bool {
	return upd.DownloadDone && upd.MinioShutdownDone && upd.SystemdUpdateDone
}

func (rel *MinioRelease) GetUpdate(cli *client.EtcdClient, prefix string, pools *MinioServerPools) (*ReleaseUpdate, error) {
	downloadKey, shutdownKey, systemdKey := rel.getTaskKeys(prefix)

	for _, key := range []string{downloadKey, shutdownKey, systemdKey} {
		tk, _, err := GetTask(cli, key)
		if err != nil {
			return nil, err
		}

		if !tk.CanContinue(pools) {
			return &ReleaseUpdate{
				DownloadDone:      key != downloadKey,
				MinioShutdownDone: key != downloadKey && key != shutdownKey,
				SystemdUpdateDone: false,
				CurrentTaskStatus: tk,
			}, nil
		}
	}

	return &ReleaseUpdate{
		DownloadDone:      true,
		MinioShutdownDone: true,
		SystemdUpdateDone: true,
		CurrentTaskStatus: nil,
	}, nil
}

func (upd *ReleaseUpdate) HandleNextTask(cli *client.EtcdClient, prefix string, rel *MinioRelease, pools *MinioServerPools, host string, action TaskAction) error {	
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

	downloadKey, shutdownKey, systemdKey := rel.getTaskKeys(prefix)
	if !upd.DownloadDone {
		upd.DownloadDone = true
		tk, _, err := GetTask(cli, downloadKey)
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

type ReleaseChangeAction func(*MinioRelease, *MinioServerPools) error

func HandleReleaseChanges(cli *client.EtcdClient, prefix string, startRel *MinioRelease, action ReleaseChangeAction, log logger.Logger) <-chan error {
	errCh := make(chan error)
	go func() {
		log.Infof("[etcd] Starting to watch for minio release changes")

		configKey := fmt.Sprintf(ETCD_RELEASE_CONFIG_KEY, prefix)

		rel, rev, getErr := GetMinioRelease(cli, prefix)
		if getErr != nil {
			errCh <- getErr
			close(errCh)
			return
		}

		if (*rel).Version != (*startRel).Version {
			log.Infof("[etcd] Handling new minio release at version %s", (*rel).Version)
			
			pools, _, getErr := GetMinioServerPools(cli, prefix)
			if getErr != nil {
				errCh <- getErr
				close(errCh)
				return
			}

			actErr := action(rel, pools)
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
			rel := MinioRelease{}

			if info.Error != nil {
				errCh <- info.Error
				close(errCh)
				return
			}

			if len(info.Changes.Deletions) > 0 {
				errCh <- errors.New("Got an unexpected etcd key deletion while looking for release changes")
				close(errCh)
				return
			}

			err := yaml.Unmarshal([]byte(info.Changes.Upserts[configKey].Value), &rel)
			if err != nil {
				errCh <- errors.New(fmt.Sprintf("Error parsing the minio release configuration: %s", err.Error()))
				close(errCh)
				return
			}

			log.Infof("[etcd] Handling new minio release at version %s", rel.Version)

			pools, _, getErr := GetMinioServerPools(cli, prefix)
			if getErr != nil {
				errCh <- getErr
				close(errCh)
				return
			}

			actErr := action(&rel, pools)
			if actErr != nil {
				errCh <- actErr
				close(errCh)
				return
			}
		}
	}()
	return errCh
}