package main

import (
	"os"

	"github.com/Ferlab-Ste-Justine/ferio/binary"
	"github.com/Ferlab-Ste-Justine/ferio/config"
	"github.com/Ferlab-Ste-Justine/ferio/etcd"
	"github.com/Ferlab-Ste-Justine/ferio/fs"
	"github.com/Ferlab-Ste-Justine/ferio/logger"
	"github.com/Ferlab-Ste-Justine/ferio/systemd"
	"github.com/Ferlab-Ste-Justine/ferio/update"
	"github.com/Ferlab-Ste-Justine/ferio/utils"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func EnsureBinariesDirExist(binsDir string, log logger.Logger) error {
	binDirExists, binDirExistsErr := fs.PathExists(binsDir)
	if binDirExistsErr != nil {
		return binDirExistsErr
	}
	if !binDirExists {
		log.Infof("[main] Creating binary directorty %s", binsDir)
		mkdirErr := os.MkdirAll(binsDir, 0700)
		if mkdirErr != nil {
			return mkdirErr
		}
	}

	return nil
}

func Startup(cli *client.EtcdClient, conf config.Config, log logger.Logger) (*etcd.MinioServerPools, *etcd.MinioRelease, error) {	
	pools, _, poolsErr := etcd.GetMinioServerPools(cli, conf.Etcd.ConfigPrefix)
	if poolsErr != nil {
		return nil, nil, poolsErr
	}

	rel, _, relErr := etcd.GetMinioRelease(cli, conf.Etcd.ConfigPrefix)
	if relErr != nil {
		return nil, nil, relErr
	}

	serviceExists, serviceExistsErr := systemd.MinioServiceExists()
	if serviceExistsErr != nil {
		return nil, nil, serviceExistsErr
	}

	minPath := binary.GetMinioPathFromVersion(conf.BinariesDir, rel.Version)

	if !serviceExists {
		log.Infof("[main] Minio service not found. Will generate it")
		downErr := binary.GetBinary(rel.Url, rel.Version, rel.Checksum, conf.BinariesDir)
		if downErr != nil {
			return nil, nil, downErr
		}

		refrErr := systemd.RefreshMinioSystemdUnit(minPath, pools)
		if refrErr != nil {
			return nil, nil, refrErr
		}
	}

	_, updErr := update.UpdatePools(cli, conf.Etcd.WorkspacePrefix, minPath, pools, conf.Host)
	if updErr != nil {
		return nil, nil, updErr
	}

	updatedRelease, updRelErr := update.UpdateRelease(cli, conf.Etcd.WorkspacePrefix, conf.BinariesDir, rel, pools, conf.Host)
	if updRelErr != nil {
		return nil, nil, updRelErr
	}

	startErr := systemd.StartMinio()
	if startErr != nil {
		return nil, nil, startErr
	}

	if updatedRelease {
		cleanupErr := binary.CleanupOldBinaries(conf.BinariesDir)
		if cleanupErr != nil {
			return nil, nil, cleanupErr
		}
	}

	return pools, rel, nil
}

func RuntimeLoop(cli *client.EtcdClient, conf config.Config, startPools *etcd.MinioServerPools, startRel *etcd.MinioRelease) error {
	poolsCh := etcd.HandleServerPoolsChanges(
		cli,
		conf.Etcd.ConfigPrefix,
		startPools,
		func(newPools *etcd.MinioServerPools, currentRel *etcd.MinioRelease) error {
			minPath := binary.GetMinioPathFromVersion(conf.BinariesDir, currentRel.Version)
			_, updErr := update.UpdatePools(cli, conf.Etcd.WorkspacePrefix, minPath, newPools, conf.Host)
			if updErr != nil {
				return  updErr
			}

			startErr := systemd.StartMinio()
			if startErr != nil {
				return startErr
			}

			return nil
		},
	)
	relCh := etcd.HandleReleaseChanges(
		cli,
		conf.Etcd.ConfigPrefix,
		startRel,
		func(newRel *etcd.MinioRelease, currentPools *etcd.MinioServerPools) error {
			_, updErr := update.UpdateRelease(cli, conf.Etcd.WorkspacePrefix, conf.BinariesDir, newRel, currentPools, conf.Host)
			if updErr != nil {
				return updErr
			}
			
			startErr := systemd.StartMinio()
			if startErr != nil {
				return startErr
			}

			cleanupErr := binary.CleanupOldBinaries(conf.BinariesDir)
			if cleanupErr != nil {
				return cleanupErr
			}

			return nil
		},
	)
	
	select {
	case poolsErr := <-poolsCh:
		return poolsErr
	case relErr := <-relCh:
		return relErr
	}

	return nil

}

func main() {
	log := logger.Logger{LogLevel: logger.ERROR}

	conf, configErr := config.GetConfig()
	utils.AbortOnErr(configErr, log)

	log.LogLevel = conf.GetLogLevel()

	ensBinDirErr := EnsureBinariesDirExist(conf.BinariesDir, log)
	utils.AbortOnErr(ensBinDirErr, log)

	cli, cliErr := etcd.GetClient(conf.Etcd)
	utils.AbortOnErr(cliErr, log)
	defer cli.Close()

	startPools, startRel, StartErr := Startup(cli, conf, log)
	utils.AbortOnErr(StartErr, log)

	runtimeErr := RuntimeLoop(cli, conf, startPools, startRel)
	utils.AbortOnErr(runtimeErr, log)
}