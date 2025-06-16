package systemd

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/Ferlab-Ste-Justine/ferio/logger"
	"github.com/Ferlab-Ste-Justine/ferio/pool"

	"github.com/coreos/go-systemd/v22/dbus"
)

func getDefaultService() MinioService {
	return MinioService{Name: "minio", EnvPath: "/etc/minio/env", DataPath: ""}
}

func TestExistsRefresh(t *testing.T) {
	log := logger.Logger{LogLevel: logger.ERROR}

	defer func() {
		delErr := DeleteMinioSystemdUnit(getDefaultService(), log)
		if delErr != nil {
			t.Errorf("Error occured cleaning up minio service: %s", delErr.Error())
		}
	}()

	exists, existsErr := MinioServiceExists(getDefaultService())
	if existsErr != nil {
		t.Errorf("Error determining existence of minio service: %s", existsErr.Error())
	}

	if exists {
		t.Errorf("Expected minio service not to exist when test started")
	}

	curDir, curDirErr := os.Getwd()
	if curDirErr != nil {
		t.Errorf("Error getting current working directory: %s", curDirErr.Error())
	}
	binDir := path.Join(curDir, "test.sh")

	refErr := RefreshMinioSystemdUnit("/bin/minio", pool.MinioServerPools{}, getDefaultService(), log)
	if refErr != nil {
		t.Errorf("Error refreshing minio unit file: %s", refErr.Error())
	}

	exists, existsErr = MinioServiceExists(getDefaultService())
	if existsErr != nil {
		t.Errorf("Error determining existence of minio service: %s", existsErr.Error())
	}

	if !exists {
		t.Errorf("Expected minio service to exist after refreshing the unit file")
	}
}

func TestStartStop(t *testing.T) {
	log := logger.Logger{LogLevel: logger.ERROR}

	defer func() {
		delErr := DeleteMinioSystemdUnit(getDefaultService(), log)
		if delErr != nil {
			t.Errorf("Error occured cleaning up minio service: %s", delErr.Error())
		}
	}()

	exists, existsErr := MinioServiceExists(getDefaultService())
	if existsErr != nil {
		t.Errorf("Error determining existence of minio service: %s", existsErr.Error())
	}

	if exists {
		t.Errorf("Expected minio service not to exist when test started")
	}

	curDir, curDirErr := os.Getwd()
	if curDirErr != nil {
		t.Errorf("Error getting current working directory: %s", curDirErr.Error())
	}
	binDir := path.Join(curDir, "test.sh")

	refErr := RefreshMinioSystemdUnit("/bin/minio", pool.MinioServerPools{}, getDefaultService(), log)
	if refErr != nil {
		t.Errorf("Error refreshing minio unit file: %s", refErr.Error())
	}

	conn, connErr := dbus.NewSystemdConnectionContext(context.Background())
	if connErr != nil {
		t.Errorf("Error connecting to systemd: %s", connErr.Error())
	}
	defer conn.Close()

	startErr := StartMinioService(getDefaultService(), log)
	if startErr != nil {
		t.Errorf("Error starting mock minio: %s", startErr.Error())
	}

	statuses, listErr := conn.ListUnitsByNamesContext(context.Background(), []string{"minio.service"})
	if listErr != nil {
		t.Errorf("Error getting minio service status: %s", listErr.Error())
	}

	if statuses[0].ActiveState != "active" && statuses[0].ActiveState != "activating" {
		t.Errorf("Expected active state after start to be active or activating and it was: %s", statuses[0].ActiveState)
	}

	stopErr := StopMinioService(getDefaultService(), log)
	if stopErr != nil {
		t.Errorf("Error stopping mock minio: %s", stopErr.Error())
	}

	statuses, listErr = conn.ListUnitsByNamesContext(context.Background(), []string{"minio.service"})
	if listErr != nil {
		t.Errorf("Error getting minio service status: %s", listErr.Error())
	}

	if statuses[0].ActiveState != "inactive" || statuses[0].SubState != "dead" {
		t.Errorf("Expected active state after stop to be inactive and substatus to be dead and they were: %s (active state), %s (substate)", statuses[0].ActiveState, statuses[0].SubState)
	}
}