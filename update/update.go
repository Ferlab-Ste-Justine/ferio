package update

import (
	"github.com/Ferlab-Ste-Justine/ferio/binary"
	"github.com/Ferlab-Ste-Justine/ferio/etcd"
	"github.com/Ferlab-Ste-Justine/ferio/logger"
	"github.com/Ferlab-Ste-Justine/ferio/systemd"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func UpdatePools(cli *client.EtcdClient, prefix string, minioPath string, pools *etcd.MinioServerPools, host string, log logger.Logger) (bool, error) {
	upd, updErr := pools.GetUpdate(cli, prefix)
	if updErr != nil {
		return false, updErr
	}

	if upd.IsDone() {
		return false, nil
	}

	if !upd.AcknowledgmentDone {
		err := upd.HandleNextTask(
			cli,
			prefix,
			pools,
			host,
			func() error {
				return nil
			},
		)
		if err != nil {
			return false, err
		}
	}

	if !upd.MinioShutdownDone {
		err := upd.HandleNextTask(
			cli,
			prefix,
			pools,
			host,
			func() error {
				return systemd.StopMinio(log)
			},
		)
		if err != nil {
			return false, err
		}
	}

	if !upd.SystemdUpdateDone {
		err := upd.HandleNextTask(
			cli,
			prefix,
			pools,
			host,
			func() error {
				return systemd.RefreshMinioSystemdUnit(minioPath, pools, log)
			},
		)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func UpdateRelease(cli *client.EtcdClient, prefix string, binariesDir string, rel *etcd.MinioRelease, pools *etcd.MinioServerPools, host string, log logger.Logger) (bool, error) {
	upd, updErr := rel.GetUpdate(cli, prefix, pools)
	if updErr != nil {
		return false, updErr
	}

	if upd.IsDone() {
		return false, nil
	}

	if !upd.DownloadDone {
		err := upd.HandleNextTask(
			cli,
			prefix,
			rel,
			pools,
			host,
			func() error {
				return binary.GetBinary(rel.Url, rel.Version, rel.Checksum, binariesDir)
			},
		)
		if err != nil {
			return false, err
		}
	}

	if !upd.MinioShutdownDone {
		err := upd.HandleNextTask(
			cli,
			prefix,
			rel,
			pools,
			host,
			func() error {
				return systemd.StopMinio(log)
			},
		)
		if err != nil {
			return false, err
		}
	}

	if !upd.SystemdUpdateDone {
		err := upd.HandleNextTask(
			cli,
			prefix,
			rel,
			pools,
			host,
			func() error {
				return systemd.RefreshMinioSystemdUnit(binary.GetMinioPathFromVersion(binariesDir, rel.Version), pools, log)
			},
		)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}