package etcd

import (
	"errors"
	"fmt"
	yaml "gopkg.in/yaml.v2"

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

type ReleaseUpdateStatus struct {
	DownloadDone       bool
	MinioShutdownDone  bool
	SystemdUpdateDone  bool
	CurrentTaskStatus  *Task
}

func (status *ReleaseUpdateStatus) IsDone() bool {
	return status.DownloadDone && status.MinioShutdownDone && status.SystemdUpdateDone
}

func (rel *MinioRelease) GetUpdateStatus(cli *client.EtcdClient, prefix string, pools *MinioServerPools) (*ReleaseUpdateStatus, error) {
	downloadKey, shutdownKey, systemdKey := rel.getTaskKeys(prefix)

	for _, key := range []string{downloadKey, shutdownKey, systemdKey} {
		tk, _, err := GetTaskStatus(cli, key)
		if err != nil {
			return nil, err
		}

		if !tk.CanContinue(pools) {
			return &ReleaseUpdateStatus{
				DownloadDone:      key != downloadKey,
				MinioShutdownDone: key != downloadKey && key != shutdownKey,
				SystemdUpdateDone: false,
				CurrentTaskStatus: tk,
			}, nil
		}
	}

	return &ReleaseUpdateStatus{
		DownloadDone:      true,
		MinioShutdownDone: true,
		SystemdUpdateDone: true,
		CurrentTaskStatus: nil,
	}, nil
}

func (rel *MinioRelease) HandleNextTask(cli *client.EtcdClient, prefix string, status *ReleaseUpdateStatus, pools *MinioServerPools, host string, action TaskAction) error {	
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

	downloadKey, shutdownKey, systemdKey := rel.getTaskKeys(prefix)
	if !status.DownloadDone {
		status.DownloadDone = true
		tk, _, err := GetTaskStatus(cli, downloadKey)
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