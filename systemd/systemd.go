package systemd

import (
	_ "embed"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/Ferlab-Ste-Justine/ferio/fs"
	"github.com/Ferlab-Ste-Justine/ferio/logger"
)

const SYSTEMD_UNIT_FILES_PATH = "/etc/systemd/system"

var (
	//go:embed minio.service
	minioUnitTemplate string
)

type UnitFileTemplate struct {
	MinioPath string
	ServerPools string
}

func DeleteMinioSystemdUnit(log logger.Logger) error {
	exists, existsErr := fs.PathExists(path.Join(SYSTEMD_UNIT_FILES_PATH, "minio.service"))
	if existsErr != nil {
		return existsErr
	}

	if !exists {
		return nil
	}

	log.Infof("[systemd] Deleting minio unit file")

	stopErr := StopMinio(log)
	if stopErr != nil {
		return stopErr
	}

	remErr := os.Remove(path.Join(SYSTEMD_UNIT_FILES_PATH, "minio.service"))
	if remErr != nil {
		return remErr
	}

	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	reloadErr := conn.Reload()
	if reloadErr != nil {
		return reloadErr
	}

	return nil
}

func RefreshMinioSystemdUnit(tpl *UnitFileTemplate, log logger.Logger) error {
	log.Infof("[systemd] Generating minio unit file with binary path %s, server pools '%s', and reloading systemd", tpl.MinioPath, tpl.ServerPools)

	tmpl, tErr := template.New("template").Parse(minioUnitTemplate)
	if tErr != nil {
		return tErr
	}

	var b bytes.Buffer
	exErr := tmpl.Execute(&b, tpl)
	if exErr != nil {
		return exErr
	}

	unitContent := b.Bytes()
	unitPath := path.Join(SYSTEMD_UNIT_FILES_PATH, "minio.service")

	writeErr := ioutil.WriteFile(unitPath, unitContent, 0640)
	if writeErr != nil {
		return writeErr
	}

	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	reloadErr := conn.Reload()
	if reloadErr != nil {
		return reloadErr
	}

	return nil
}

func MinioServiceExists() (bool, error) {
	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return false, connErr
	}
	defer conn.Close()

	statuses, listErr := conn.ListUnitsByNamesContext(context.Background(), []string{"minio.service"})
	if listErr != nil {
		return false, listErr
	}

	return len(statuses) > 0 && statuses[0].LoadState != "not-found", nil
}

func StopMinio(log logger.Logger) error {
	log.Infof("[systemd] Stopping minio service")

	exists, existsErr := MinioServiceExists()
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		log.Infof("[systemd] Stopping aborted. Minio service does not exist")
		return nil
	}
	
	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	output := make(chan string)
	defer close(output)
	_, stopErr := conn.StopUnitContext(context.Background(), "minio.service", "replace", output)
	if stopErr != nil {
		return stopErr
	}
	res := <-output
	if res != "done" {
		return errors.New(fmt.Sprintf("Expected stopping minio service to return a result of 'done' and got %s", res))
	}

	_, disableErr := conn.DisableUnitFilesContext(context.Background(), []string{"minio.service"}, false)
	if disableErr != nil {
		return disableErr
	}

	return nil
}

func StartMinio(log logger.Logger) error {
	log.Infof("[systemd] Starting minio service")

	exists, existsErr := MinioServiceExists()
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		log.Infof("[systemd] Starting aborted. Minio service does not exist")
		return nil
	}

	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	output := make(chan string)
	defer close(output)
	_, startErr := conn.StartUnitContext(context.Background(), "minio.service", "replace", output)
	if startErr != nil {
		return startErr
	}
	res := <-output
	if res != "done" {
		return errors.New(fmt.Sprintf("Expected starting minio service to return a result of 'done' and got %s", res))
	}

	_, _, enableErr := conn.EnableUnitFilesContext(context.Background(), []string{"minio.service"}, false, true)
	if enableErr != nil {
		return enableErr
	}

	return nil
}