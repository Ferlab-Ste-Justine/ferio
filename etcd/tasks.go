package etcd

import (
	"fmt"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

const ETCD_TASK_COMPLETION_KEY = "%scomplete"
const ETCD_TASK_COMPLETERS_PREFIX = "%scompleters/"

type Task struct {
	Complete bool
	Completers []string
}

func (tk *Task) HasToDo(host string) bool {
	for _, completer := range tk.Completers {
		if host == completer {
			return false
		}
	}

	return true
}

func (tk *Task) CanContinue(hostsCount int64) bool {
	return tk.Complete || int64(len(tk.Completers)) >= hostsCount
}

func GetTask(cli *client.EtcdClient, taskPrefix string) (*Task, int64, error) {
	tk := Task{false, []string{}}
	
	members, rev, getErr := cli.GetGroupMembers(fmt.Sprintf(ETCD_TASK_COMPLETERS_PREFIX, taskPrefix))
	if getErr != nil {
		return &tk, rev, getErr
	}
	for member, _ := range members {
		tk.Completers = append(tk.Completers, member)
	}

	info, infoErr := cli.GetKey(fmt.Sprintf(ETCD_TASK_COMPLETION_KEY, taskPrefix), client.GetKeyOptions{Revision: rev})
	if infoErr != nil {
		return &tk, rev, infoErr
	}
	tk.Complete = info.Found()

	return &tk, rev, nil
}

func MarkTaskDoneBySelf(cli *client.EtcdClient, taskPrefix string, host string) error {
	return cli.JoinGroup(fmt.Sprintf(ETCD_TASK_COMPLETERS_PREFIX, taskPrefix), host, "done")
}

func WaitOnTaskCompletion(cli *client.EtcdClient, taskPrefix string, hostsCount int64) error {	
	doneCh := make(chan struct{})
	defer close(doneCh)
	
	errCh := cli.WaitGroupCountThreshold(fmt.Sprintf(ETCD_TASK_COMPLETERS_PREFIX, taskPrefix), hostsCount, doneCh)
	err := <- errCh
	if err != nil {
		return err
	}

	_, putErr := cli.PutKey(fmt.Sprintf(ETCD_TASK_COMPLETION_KEY, taskPrefix), "true")
	return putErr
}

type TaskAction func() error