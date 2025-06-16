package pool

import (
	"fmt"
	"path"
	"strings"
)

type MinioServerPool struct {
	ApiPort           int64  `yaml:"api_port"`
	DomainTemplate    string `yaml:"domain_template"`
	ServerCountBegin  int64  `yaml:"server_count_begin"`
	ServerCountEnd    int64  `yaml:"server_count_end"`
	MountPathTemplate string `yaml:"mount_path_template"`
	MountCount        int64  `yaml:"mount_count"`
}

func (pool *MinioServerPool) Stringify(dir string) string {
	urls := fmt.Sprintf(
		"https://%s:%d",
		fmt.Sprintf(
			pool.DomainTemplate,
			fmt.Sprintf("{%d...%d}", pool.ServerCountBegin, pool.ServerCountEnd),
		),
		pool.ApiPort,
	)
	mounts := fmt.Sprintf(
		pool.MountPathTemplate,
		fmt.Sprintf("{1...%d}", pool.MountCount),
	)

	res := fmt.Sprintf("%s%s", urls, mounts)

	if dir != "" {
		res = path.Join(res, dir)
	}

	return res
}

type MinioServerPools []MinioServerPool

func (pools *MinioServerPools) CountHosts() int64 {
	count := int64(0)
	for _, pool := range *pools {
		count += (pool.ServerCountEnd - pool.ServerCountBegin + 1)
	}
	return count
}

func (pools *MinioServerPools) Stringify(dir string) string {
	stringifiedPools := []string{}
	for _, pool := range *pools {
		stringifiedPools = append(stringifiedPools, pool.Stringify(dir))
	}

	return strings.Join(stringifiedPools, " ")
}