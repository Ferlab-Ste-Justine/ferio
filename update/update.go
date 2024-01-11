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
		log.Debugf("[update] Server pools update is done. Skipping it")
		return false, nil
	}

	log.Infof("[update] Detected ongoing server pools update. Will synchronize with other minio nodes to complete it")

	if !upd.AcknowledgmentDone {
		log.Debugf("[update] Synchronizing on server pools update acknowledgment")
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
		log.Debugf("[update] Synchronizing on server pools update minio shutdown")
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
		log.Debugf("[update] Synchronizing on server pools update systemd unit refresh")
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
		log.Debugf("[update] Release update is done. Skipping it")
		return false, nil
	}

	log.Infof("[update] Detected ongoing minio release update. Will synchronize with other minio nodes to complete it")

	if !upd.DownloadDone {
		log.Debugf("[update] Synchronizing on release update binary download")
		err := upd.HandleNextTask(
			cli,
			prefix,
			rel,
			pools,
			host,
			func() error {
				return binary.GetBinary(rel.Url, rel.Version, rel.Checksum, binariesDir, log)
			},
		)
		if err != nil {
			return false, err
		}
	}

	if !upd.MinioShutdownDone {
		log.Debugf("[update] Synchronizing on release update minio shutdown")
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
		log.Debugf("[update] Synchronizing on release update systemd unit refresh")
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