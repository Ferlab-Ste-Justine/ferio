package etcd

import (
	"fmt"
	"testing"
	"time"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/testutils"
)

func TestHasToDo(t *testing.T) {
	tsk := Task{Complete: false, Completers: []string{}}
	if !tsk.HasToDo("test") {
		t.Errorf("Expected to have to do empty task, but didn't")
	}
	
	tsk.Complete = true
	if !tsk.HasToDo("host1") {
		t.Errorf("Expected to have to do empty task, but didn't")
	}

	tsk.Completers = []string{"host1", "host2", "host3"}
	if tsk.HasToDo("host1") {
		t.Errorf("Expected not to have to do a task where host is part of completers, but did")
	}

	if !tsk.HasToDo("host4") {
		t.Errorf("Expected to have to do a task where host is not part of completers, but didn't")
	}
}

func TestCanContinue(t *testing.T) {
	tsk := Task{Complete: false, Completers: []string{"host1", "host2", "host3"}}
	
	if !tsk.CanContinue(3) {
		t.Errorf("Expected to be able to continue when the number of completers match the hosts count")
	}

	if !tsk.CanContinue(2) {
		t.Errorf("Expected to be able to continue when the number of completers is greater than the hosts count")
	}

	if tsk.CanContinue(4) {
		t.Errorf("Expected not to be able to continue when the number of completers is less than the hosts count and the complete flag is at false")
	}

	tsk.Complete = true

	if !tsk.CanContinue(2) {
		t.Errorf("Expected to be able to continue when the complete flag is at true")
	}

	if !tsk.CanContinue(4) {
		t.Errorf("Expected to be able to continue when the complete flag is at true")
	}
}

func TestGetTask(t *testing.T) {
	tearDown, launchErr := testutils.LaunchTestEtcdCluster("../test", testutils.EtcdTestClusterOpts{})
	if launchErr != nil {
		t.Errorf("Error occured launching test etcd cluster: %s", launchErr.Error())
		return
	}

	defer func() {
		errs := tearDown()
		if len(errs) > 0 {
			t.Errorf("Errors occured tearing down etcd cluster: %s", errs[0].Error())
		}
	}()

	retryInterval, _ := time.ParseDuration("1s")
	timeouts, _ := time.ParseDuration("10s")
	retries := uint64(10)
	cli := setupTestEnv(t, timeouts, retryInterval, retries)

	tsk, _, err := GetTask(cli, "/task1/")
	if err != nil {
		t.Errorf("Error occured getting task: %s", err.Error())
	}
	
	if tsk.Completers == nil || len(tsk.Completers) != 0 {
		t.Errorf("Expected completers on an empty key space to be an empty array and that was not the case")
	}

	if tsk.Complete {
		t.Errorf("Expected complete flag on an empty key space to be false and that was not the case")	
	}

	_, err = cli.PutKey("/task1/completers/host1", "done")
	if err != nil {
		t.Errorf("Error occured adding host to completers: %s", err.Error())
	}

	_, err = cli.PutKey("/task1/completers/host2", "done")
	if err != nil {
		t.Errorf("Error occured adding host to completers: %s", err.Error())
	}

	tsk, _, err = GetTask(cli, "/task1/")
	if err != nil {
		t.Errorf("Error occured getting task: %s", err.Error())
	}

	if len(tsk.Completers) != 2 || (!isStringInSlice("host1", tsk.Completers)) || (!isStringInSlice("host2", tsk.Completers)) {
		t.Errorf("Expected exactly host1 and host2 to be in completers on keyspace with only host1 and host2 and that was not the case")
	}

	if tsk.Complete {
		t.Errorf("Expected complete flag to be false on keyspace with only host1 and host2 and that was not the case")	
	}

	_, err = cli.PutKey("/task1/complete", "true")
	if err != nil {
		t.Errorf("Error occured setting task to complete: %s", err.Error())
	}

	tsk, _, err = GetTask(cli, "/task1/")
	if err != nil {
		t.Errorf("Error occured getting task: %s", err.Error())
	}

	if len(tsk.Completers) != 2 || (!isStringInSlice("host1", tsk.Completers)) || (!isStringInSlice("host2", tsk.Completers)) {
		t.Errorf("Expected exactly host1 and host2 to be in completers on keyspace with host1 and host2 and marked as complete and that was not the case")
	}

	if !tsk.Complete {
		t.Errorf("Expected complete flag to be true on keyspace with host1 and host2 and marked as complete and that was not the case")	
	}
}

func TestMarkTaskDoneBySelf(t *testing.T) {
	tearDown, launchErr := testutils.LaunchTestEtcdCluster("../test", testutils.EtcdTestClusterOpts{})
	if launchErr != nil {
		t.Errorf("Error occured launching test etcd cluster: %s", launchErr.Error())
		return
	}

	defer func() {
		errs := tearDown()
		if len(errs) > 0 {
			t.Errorf("Errors occured tearing down etcd cluster: %s", errs[0].Error())
		}
	}()

	retryInterval, _ := time.ParseDuration("1s")
	timeouts, _ := time.ParseDuration("10s")
	retries := uint64(10)
	cli := setupTestEnv(t, timeouts, retryInterval, retries)

	err := MarkTaskDoneBySelf(cli, "/task1/", "host1")
	if err != nil {
		t.Errorf("Error occured marking task as done by a host: %s", err.Error())
	}

	tsk, _, tskErr := GetTask(cli, "/task1/")
	if tskErr != nil {
		t.Errorf("Error occured getting task: %s", tskErr.Error())
	}

	if len(tsk.Completers) != 1 || (!isStringInSlice("host1", tsk.Completers)) {
		t.Errorf("Expected exactly host1 to be in completers on keyspace with host1 and that was not the case")
	}

	if tsk.Complete {
		t.Errorf("Expected complete flag to be false on keyspace with host1 and that was not the case")	
	}

	err = MarkTaskDoneBySelf(cli, "/task1/", "host2")
	if err != nil {
		t.Errorf("Error occured marking task as done by a host: %s", err.Error())
	}

	tsk, _, tskErr = GetTask(cli, "/task1/")
	if tskErr != nil {
		t.Errorf("Error occured getting task: %s", tskErr.Error())
	}

	if len(tsk.Completers) != 2 || (!isStringInSlice("host1", tsk.Completers)) || (!isStringInSlice("host2", tsk.Completers)) {
		t.Errorf("Expected exactly host1 and host2 to be in completers on keyspace with host1 and host2 and that was not the case")
	}

	if tsk.Complete {
		t.Errorf("Expected complete flag to be false on keyspace with host1 and host2 and that was not the case")	
	}

	_, err = cli.PutKey("/task1/complete", "true")
	if err != nil {
		t.Errorf("Error occured setting task to complete: %s", err.Error())
	}

	err = MarkTaskDoneBySelf(cli, "/task1/", "host3")
	if err != nil {
		t.Errorf("Error occured marking task as done by a host: %s", err.Error())
	}

	tsk, _, tskErr = GetTask(cli, "/task1/")
	if tskErr != nil {
		t.Errorf("Error occured getting task: %s", tskErr.Error())
	}

	if len(tsk.Completers) != 3 || (!isStringInSlice("host1", tsk.Completers)) || (!isStringInSlice("host2", tsk.Completers)) || (!isStringInSlice("host3", tsk.Completers)) {
		t.Errorf("Expected exactly host1, host2 and host3 to be in completers on keyspace with host1 and host2 and host3 and marked as complete and that was not the case")
	}

	if !tsk.Complete {
		t.Errorf("Expected complete flag to be true on keyspace with host1, host2 and host3 that is marked as complete that was not the case")	
	}
}

func TestWaitOnTaskCompletion(t *testing.T) {
	tearDown, launchErr := testutils.LaunchTestEtcdCluster("../test", testutils.EtcdTestClusterOpts{})
	if launchErr != nil {
		t.Errorf("Error occured launching test etcd cluster: %s", launchErr.Error())
		return
	}

	defer func() {
		errs := tearDown()
		if len(errs) > 0 {
			t.Errorf("Errors occured tearing down etcd cluster: %s", errs[0].Error())
		}
	}()

	retryInterval, _ := time.ParseDuration("1s")
	timeouts, _ := time.ParseDuration("10s")
	retries := uint64(10)
	cli := setupTestEnv(t, timeouts, retryInterval, retries)

	go func() {
		for idx := 1; idx < 31; idx++ {
			err := MarkTaskDoneBySelf(cli, "/task1/", fmt.Sprintf("host%d", idx))
			if err != nil {
				t.Errorf("Error occured marking task as done by a host: %s", err.Error())
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	err := WaitOnTaskCompletion(cli, "/task1/", 30)
	if err != nil {
		t.Errorf("Error occured waiting on task completions: %s", err.Error())
	}

	tsk, _, tskErr := GetTask(cli, "/task1/")
	if tskErr != nil {
		t.Errorf("Error occured getting task: %s", tskErr.Error())
	}

	if len(tsk.Completers) != 30 {
		t.Errorf("Expected number of completers after waiting for task completion to be 30 and it was not the case")
	}

	if !tsk.Complete {
		t.Errorf("Expected task to be marked as complete after waiting for task completion and it was not the case")	
	}
}