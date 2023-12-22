package fs

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
)

func PathExists(fsPath string) (bool, error) {
	_, err := os.Stat(fsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return true, err
		}

		return false, nil
	}

	return true, nil
}

func GetFileSha256(src string) (string, error) {
	fHandle, handleErr := os.Open(src)
	if handleErr != nil {
	  return "", handleErr
	}
	defer fHandle.Close()
  
	hash := sha256.New()
	_, copyErr := io.Copy(hash, fHandle)
	if copyErr != nil {
		return "", copyErr
	}
  
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func GetTopSubDirectories(parentDir string) ([]string, error) {
    list, listErr := os.ReadDir(parentDir)
    if listErr != nil {
        return nil, listErr
    }
 
	dirs := []string{}
	for _, dir := range list {
		dirs = append(dirs, path.Join(parentDir, dir.Name()))
	}

	return dirs, nil
}

func KeepLastDirectories(amount int64, dirs []string) error {
	if int64(len(dirs)) <= amount {
		return nil
	}

	toTrim := int64(len(dirs)) - amount
	for idx, dir := range dirs {
		if int64(idx) >= toTrim {
			break
		}

		rmErr := os.RemoveAll(dir)
		if rmErr != nil {
			return rmErr
		}
	}

	return nil
}