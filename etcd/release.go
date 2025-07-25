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

const ETCD_RELEASE_TASKS_BINARY_DOWNLOAD_KEY = "%stasks/release/%s/binary_download/"
const ETCD_RELEASE_TASKS_MINIO_SHUTDOWN_KEY = "%stasks/release/%s/minio_shutdown/"
const ETCD_RELEASE_TASKS_SYSTEMD_UPDATE_KEY = "%stasks/release/%s/systemd_update/"

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

func (upd *ReleaseUpdate) GetTaskKey(prefix string, rel *MinioRelease) string  {
	if !upd.DownloadDone {
		return fmt.Sprintf(ETCD_RELEASE_TASKS_BINARY_DOWNLOAD_KEY, prefix, rel.Version)
	} else if !upd.MinioShutdownDone {
		return fmt.Sprintf(ETCD_RELEASE_TASKS_MINIO_SHUTDOWN_KEY, prefix, rel.Version)
	}

	return fmt.Sprintf(ETCD_RELEASE_TASKS_SYSTEMD_UPDATE_KEY, prefix, rel.Version)
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

		if !tk.CanContinue(pools.Pools.CountHosts()) {
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
	tkKey := upd.GetTaskKey(prefix, rel)
	
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

	_, shutdownKey, systemdKey := rel.getTaskKeys(prefix)
	if !upd.DownloadDone {
		upd.DownloadDone = true
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