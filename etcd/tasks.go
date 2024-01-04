package etcd

import (
	"fmt"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

const ETCD_TASK_COMPLETION_KEY = "%s/complete"
const ETCD_TASK_COMPLETERS_PREFIX = "%s/completers/"

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

func (tk *Task) CanContinue(pools *MinioServerPools) bool {
	return tk.Complete || int64(len(tk.Completers)) == pools.CountHosts()
}

func GetTaskStatus(cli *client.EtcdClient, taskPrefix string) (*Task, int64, error) {
	tk := Task{false, []string{}}
	
	gr := client.Group{KeyPrefix: fmt.Sprintf(ETCD_TASK_COMPLETERS_PREFIX, taskPrefix)}
	
	members, rev, getErr := cli.GetGroupMembers(gr)
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
	gr := client.Group{KeyPrefix: fmt.Sprintf(ETCD_TASK_COMPLETERS_PREFIX, taskPrefix), Id: host}
	return cli.JoinGroup(gr)
}

func WaitOnTaskCompletion(cli *client.EtcdClient, taskPrefix string, hostsCount int64) error {
	gr := client.Group{KeyPrefix: fmt.Sprintf(ETCD_TASK_COMPLETERS_PREFIX, taskPrefix)}
	
	doneCh := make(chan struct{})
	defer close(doneCh)
	
	errCh := cli.WaitGroupCountThreshold(gr, hostsCount, doneCh)
	err := <- errCh
	if err != nil {
		return err
	}

	_, putErr := cli.PutKey(fmt.Sprintf(ETCD_TASK_COMPLETION_KEY, taskPrefix), "true")
	return putErr
}

type TaskAction func() error