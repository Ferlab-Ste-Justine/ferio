package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Ferlab-Ste-Justine/ferio/binary"
	"github.com/Ferlab-Ste-Justine/ferio/config"
	"github.com/Ferlab-Ste-Justine/ferio/etcd"
	"github.com/Ferlab-Ste-Justine/ferio/fs"
	"github.com/Ferlab-Ste-Justine/ferio/systemd"
	"github.com/Ferlab-Ste-Justine/ferio/update"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

func EnsureBinariesDirExist(binsDir string) error {
	binDirExists, binDirExistsErr := fs.PathExists(binsDir)
	if binDirExistsErr != nil {
		return binDirExistsErr
	}
	if !binDirExists {
		mkdirErr := os.MkdirAll(binsDir, 0700)
		if mkdirErr != nil {
			return mkdirErr
		}
	}

	return nil
}

func CleanupBinariesDir(binsDir string, conf config.BinariesCleanupConfig) {
	go func() {
		for true {
			binDirs, binDirsErr := fs.GetTopSubDirectories(binsDir)
			if binDirsErr != nil {
				fmt.Printf("Error cleaning up minio binaries: %s", binDirsErr.Error())
				time.Sleep(100 * time.Minute)
				continue
			}

			cleanupErr := fs.KeepLastDirectories(conf.MaximumBinaries, binDirs)
			if cleanupErr != nil {
				fmt.Printf("Error cleaning up minio binaries: %s", cleanupErr.Error())
				time.Sleep(100 * time.Minute)
				continue
			}

			time.Sleep(conf.Interval)
		}
	}()
}

func Startup(cli *client.EtcdClient, conf config.Config) (*etcd.MinioServerPools, *etcd.MinioRelease, error) {
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

	_, updErr = update.UpdateRelease(cli, conf.Etcd.WorkspacePrefix, conf.BinariesDir, rel, pools, conf.Host)
	if updErr != nil {
		return nil, nil, updErr
	}

	startErr := systemd.StartMinio()
	if startErr != nil {
		return nil, nil, startErr
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
	conf, configErr := config.GetConfig()
	if configErr != nil {
		fmt.Println(configErr.Error())
		os.Exit(1)
	}

	ensBinDirErr := EnsureBinariesDirExist(conf.BinariesDir)
	if ensBinDirErr != nil {
		fmt.Println(ensBinDirErr.Error())
		os.Exit(1)
	}

	CleanupBinariesDir(conf.BinariesDir, conf.BinariesCleanup)

	cli, cliErr := etcd.GetClient(conf.Etcd)
	if cliErr != nil {
		fmt.Println(cliErr.Error())
		os.Exit(1)
	}
	defer cli.Close()

	startPools, startRel, err := Startup(cli, conf)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = RuntimeLoop(cli, conf, startPools, startRel)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}