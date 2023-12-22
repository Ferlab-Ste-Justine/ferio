package main

import (
	"fmt"
	"os"
	"time"

	//"github.com/Ferlab-Ste-Justine/ferio/binary"
	"github.com/Ferlab-Ste-Justine/ferio/config"
	"github.com/Ferlab-Ste-Justine/ferio/etcd"
	"github.com/Ferlab-Ste-Justine/ferio/fs"
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

	/*time.Sleep(10 * time.Second)
	getBinErr := binary.GetBinary(
		"https://dl.min.io/server/minio/release/linux-amd64/minio.RELEASE.2023-12-20T01-00-02Z", 
		"2023-12-20",
		"09fafaf399885b4912bafda6fa03fc4ccbc39ec45e17239677217317915d6aeb",
		conf.BinariesDir,
	)
	if getBinErr != nil {
		fmt.Println(getBinErr.Error())
		os.Exit(1)
	}

	getBinErr = binary.GetBinary(
		"https://dl.min.io/server/minio/release/linux-amd64/minio.RELEASE.2023-12-20T01-00-02Z", 
		"2023-11-19",
		"09fafaf399885b4912bafda6fa03fc4ccbc39ec45e17239677217317915d6aeb",
		conf.BinariesDir,
	)
	if getBinErr != nil {
		fmt.Println(getBinErr.Error())
		os.Exit(1)
	}

	getBinErr = binary.GetBinary(
		"https://dl.min.io/server/minio/release/linux-amd64/minio.RELEASE.2023-12-20T01-00-02Z", 
		"2022-11-19",
		"09fafaf399885b4912bafda6fa03fc4ccbc39ec45e17239677217317915d6aeb",
		conf.BinariesDir,
	)
	if getBinErr != nil {
		fmt.Println(getBinErr.Error())
		os.Exit(1)
	}

	minPath, minPathErr := binary.GetMinioPath(conf.BinariesDir)
	if minPathErr != nil {
		fmt.Println(minPathErr.Error())
		os.Exit(1)
	}

	fmt.Println(minPath)*/
}