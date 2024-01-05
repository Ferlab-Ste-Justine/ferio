package binary

import (
	"errors"
	"fmt"
	"io"
    "net/http"
	"os"
	"path"

	"github.com/Ferlab-Ste-Justine/ferio/fs"
)

func downloadBinary(binaryUrl string, binaryPath string, retries int) error {
	cli := http.Client{}
	resClosed := false
	fsClosed := false

	res, getErr := cli.Get(binaryUrl)
	if getErr != nil {
		if retries > 0 {
			return downloadBinary(binaryUrl, binaryPath, retries - 1)
		}

		return errors.New(fmt.Sprintf("Error downloading minio: %s", getErr.Error()))
	}
	defer func() {
		if !resClosed{
			res.Body.Close()
		}
	}()
	
	if res.StatusCode >= 400 {
		if retries > 0 {
			res.Body.Close()
			resClosed = true
			return downloadBinary(binaryUrl, binaryPath, retries - 1)
		}

		return errors.New(fmt.Sprintf("Error downloading minio: Server returned error code %d", res.StatusCode))
	}

	fsWr, fsErr := os.Create(binaryPath)
	if fsErr != nil {
		return errors.New(fmt.Sprintf("Error opening writable binary file to download minio: %s", fsErr.Error()))
	}
	defer func() {
		if !fsClosed{
			fsWr.Close()
		}
	}()

	_, cpErr := io.Copy(fsWr, res.Body)
	if cpErr != nil {
		if retries > 0 {
			res.Body.Close()
			resClosed = true
			fsWr.Close()
			fsClosed = true
			return downloadBinary(binaryUrl, binaryPath, retries - 1)
		}

		return errors.New(fmt.Sprintf("Error downloading minio in writable file: %s", cpErr.Error()))
	}

	return nil
}

func GetMinioPathFromVersion(binariesDir string, minioVersion string) string {
	return path.Join(binariesDir, minioVersion, "minio")
}

func GetBinary(minioUrl string, minioVersion string, expectedSha string, binariesDir string) error {
	binDir := path.Join(binariesDir, minioVersion)
	binPath := path.Join(binDir, "minio")
	
	exists, existsErr := fs.PathExists(binPath)
	if existsErr != nil {
		return errors.New(fmt.Sprintf("Error determining if minio download already exists: %s", existsErr.Error()))
	}

	if exists {
		sha, shaErr := fs.GetFileSha256(binPath)
		if shaErr != nil {
			return errors.New(fmt.Sprintf("Error checking checksum of pre-existing minio download: %s", shaErr.Error()))
		}

		if sha == expectedSha {
			return nil
		}

		removeErr := os.Remove(binPath)
		if removeErr != nil {
			return errors.New(fmt.Sprintf("Error removing bad pre-existing minio download: %s", removeErr.Error()))
		}
	}

	mkdirErr := os.MkdirAll(binDir, 0700)
	if mkdirErr != nil {
		return errors.New(fmt.Sprintf("Error creating minio download path: %s", mkdirErr.Error()))
	}

	dlErr := downloadBinary(minioUrl, binPath, 3)
	if dlErr != nil {
		return dlErr
	}

	binSha, binShaErr := fs.GetFileSha256(binPath)
	if binShaErr != nil {
		return errors.New(fmt.Sprintf("Error reading downloaded binary to check checksum: %s", binShaErr.Error()))
	}
    
	if binSha != expectedSha {
		return errors.New(fmt.Sprintf("Error downloaded binary checksum did not match expected value: %s != %s", binSha, expectedSha))
	}

	return nil
}

func GetMinioPath(binariesDir string) (string, error) {
	binDirs, binDirsErr := fs.GetTopSubDirectories(binariesDir)
	if binDirsErr != nil {
		return "", errors.New(fmt.Sprintf("Error occured while fetching the last minio binary: %s", binDirsErr.Error()))
	}

	if len(binDirs) == 0 {
		return "", nil
	}

	return path.Join(binDirs[len(binDirs) - 1], "minio"), nil
}

func CleanupOldBinaries(binariesDir string) error {
	binDirs, binDirsErr := fs.GetTopSubDirectories(binariesDir)
	if binDirsErr != nil {
		return errors.New(fmt.Sprintf("Error cleaning up minio binaries: %s", binDirsErr.Error()))
	}

	cleanupErr := fs.KeepLastDirectories(1, binDirs)
	if cleanupErr != nil {
		return errors.New(fmt.Sprintf("Error cleaning up minio binaries: %s", cleanupErr.Error()))
	}

	return nil
}