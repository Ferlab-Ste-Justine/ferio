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
	"strings"
	"text/template"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/Ferlab-Ste-Justine/ferio/fs"
	"github.com/Ferlab-Ste-Justine/ferio/logger"
	"github.com/Ferlab-Ste-Justine/ferio/pool"
)

const SYSTEMD_UNIT_FILES_PATH = "/etc/systemd/system"

var (
	//go:embed minio.service
	minioUnitTemplate string
)

type MinioService struct {
	Name     string
	DataPath string `yaml:"data_path"`
	EnvPath  string `yaml:"env_path"`
}

func (service *MinioService) GetUnitName() string {
	if strings.HasSuffix(service.Name, ".service") {
		return service.Name
	}

	return service.Name + ".service"
}

type UnitFileTemplate struct {
	MinioPath string
	EnvPath string
	ServerPools string
}

func DeleteMinioSystemdUnit(service MinioService, log logger.Logger) error {
	exists, existsErr := fs.PathExists(path.Join(SYSTEMD_UNIT_FILES_PATH, service.GetUnitName()))
	if existsErr != nil {
		return existsErr
	}

	if !exists {
		return nil
	}

	log.Infof("[systemd] Deleting %s unit file", service.GetUnitName())

	stopErr := StopMinioService(service, log)
	if stopErr != nil {
		return stopErr
	}

	remErr := os.Remove(path.Join(SYSTEMD_UNIT_FILES_PATH, service.GetUnitName()))
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

func DeleteMinioSystemdUnits(services []MinioService, log logger.Logger) error {
	for _, service := range services {
		err := DeleteMinioSystemdUnit(service, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func RefreshMinioSystemdUnit(minioPath string, pools pool.MinioServerPools, service MinioService, log logger.Logger) error {
	log.Infof("[systemd] Generating %s unit file with binary path %s, server pools '%s', and reloading systemd", service.Name, minioPath, pools.Stringify(service.DataPath))

	tmpl, tErr := template.New("template").Parse(minioUnitTemplate)
	if tErr != nil {
		return tErr
	}

	tpl := &UnitFileTemplate{
		MinioPath: minioPath,
		EnvPath: service.EnvPath,
		ServerPools: pools.Stringify(service.DataPath),
	}

	var b bytes.Buffer
	exErr := tmpl.Execute(&b, tpl)
	if exErr != nil {
		return exErr
	}

	unitContent := b.Bytes()
	unitPath := path.Join(SYSTEMD_UNIT_FILES_PATH, service.GetUnitName())

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

func RefreshMinioSystemdUnits(minioPath string, pools pool.MinioServerPools, services []MinioService, log logger.Logger) error {
	for _, service := range services {
		err := RefreshMinioSystemdUnit(minioPath, pools, service, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func MinioServiceExists(service MinioService) (bool, error) {
	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return false, connErr
	}
	defer conn.Close()

	statuses, listErr := conn.ListUnitsByNamesContext(context.Background(), []string{service.GetUnitName()})
	if listErr != nil {
		return false, listErr
	}

	return len(statuses) > 0 && statuses[0].LoadState != "not-found", nil
}

func MinioServicesExists(services []MinioService) (bool, error) {
	for _, service := range services {
		exists, err := MinioServiceExists(service)
		if err != nil {
			return false, err
		}

		if !exists {
			return false, nil
		}
	}

	return true, nil
}

func StopMinioService(service MinioService, log logger.Logger) error {
	log.Infof("[systemd] Stopping %s unit", service.GetUnitName())

	exists, existsErr := MinioServiceExists(service)
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		log.Infof("[systemd] Stopping aborted. %s unit does not exist", service.GetUnitName())
		return nil
	}
	
	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	output := make(chan string)
	defer close(output)
	_, stopErr := conn.StopUnitContext(context.Background(), service.GetUnitName(), "replace", output)
	if stopErr != nil {
		return stopErr
	}
	res := <-output
	if res != "done" {
		return errors.New(fmt.Sprintf("Expected stopping %s unit to return a result of 'done' and got %s", service.Name, res))
	}

	_, disableErr := conn.DisableUnitFilesContext(context.Background(), []string{service.GetUnitName()}, false)
	if disableErr != nil {
		return disableErr
	}

	return nil
}

func StopMinioServices(services []MinioService, log logger.Logger) error {
	for _, service := range services {
		err := StopMinioService(service, log)
		if err != nil {
			return err
		}
	}

	return nil
}

func StartMinioService(service MinioService, log logger.Logger) error {
	log.Infof("[systemd] Starting %s unit", service.GetUnitName())

	exists, existsErr := MinioServiceExists(service)
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		log.Infof("[systemd] Starting aborted. %s unit does not exist", service.GetUnitName())
		return nil
	}

	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	output := make(chan string)
	defer close(output)
	_, startErr := conn.StartUnitContext(context.Background(), service.GetUnitName(), "replace", output)
	if startErr != nil {
		return startErr
	}
	res := <-output
	if res != "done" {
		return errors.New(fmt.Sprintf("Expected starting %s unit to return a result of 'done' and got %s", service.GetUnitName(), res))
	}

	_, _, enableErr := conn.EnableUnitFilesContext(context.Background(), []string{service.GetUnitName()}, false, true)
	if enableErr != nil {
		return enableErr
	}

	return nil
}

func StartMinioServices(services []MinioService, log logger.Logger) error {
	for _, service := range services {
		err := StartMinioService(service, log)
		if err != nil {
			return err
		}
	}

	return nil
}