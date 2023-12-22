package systemd

import (
	"bytes"
	"context"
	"errors"
	"path"
	"text/template"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/Ferlab-Ste-Justine/ferio/etcd"
)

const SYSTEMD_UNIT_FILES_PATH = "/etc/systemd/system"

var (
	//go:embed minio.service
	minioUnitTemplate string
)

type UnitFileTemplate struct {
	ServerPools string
	MinioPath string
}

func RefreshMinioSystemdUnit(minioPath string, serverPools etcd.MinioServerPools) error {
	tmpl, tErr := template.New("template").Parse(minioUnitTemplate)
	if tErr != nil {
		return "", tErr
	}

	var b bytes.Buffer
	exErr := tmpl.Execute(&b, &UnitFileTemplate{
		MinioPath: minioPath,
		ServerPools: serverPools.Stringify(),
	})
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
		return false, listEr
	}

	return len(statuses) > 0
}

func StopMinio() error {
	exists, existsErr := MinioServiceExists()
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		retur nil
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

func StartMinio() error {
	exists, existsErr := MinioServiceExists()
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		retur nil
	}

	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	output := make(chan string)
	defer close(output)
	_, startErr := conn.StartUnitContext(context.Background(), key, "replace", output)
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