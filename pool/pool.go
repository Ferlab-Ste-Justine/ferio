package pool

import (
	"fmt"
	"strings"
)

type ServerPoolTenant struct {
	Tenant   string
	ApiPort  int64
	DataPath string
}

type MinioServerPool struct {
	ApiPort           int64              `yaml:"api_port"`
	Tenants           []ServerPoolTenant
	DomainTemplate    string             `yaml:"domain_template"`
	ServerCountBegin  int64              `yaml:"server_count_begin"`
	ServerCountEnd    int64              `yaml:"server_count_end"`
	MountPathTemplate string             `yaml:"mount_path_template"`
	MountCount        int64              `yaml:"mount_count"`
}

func joinPoolDir(pool string, dir string) string {
	if strings.HasPrefix(dir, "/") {
		return fmt.Sprintf("%s%s", pool, dir)
	} else {
		return fmt.Sprintf("%s/%s", pool, dir)
	}
}

func (pool *MinioServerPool) getTenant(tenant string) ServerPoolTenant {
	if tenant == "" {
		return ServerPoolTenant{
			Tenant: "",
			ApiPort: pool.ApiPort,
			DataPath: "",
		}
	}

	for _, poolTenant := range pool.Tenants {
		if tenant == poolTenant.Tenant {
			return poolTenant
		}
	}

	return ServerPoolTenant{
		Tenant: "",
		ApiPort: pool.ApiPort,
		DataPath: "",
	}
}

func (pool *MinioServerPool) Stringify(tenant string) string {
	poolTenant := pool.getTenant(tenant)

	urls := fmt.Sprintf(
		"https://%s:%d",
		fmt.Sprintf(
			pool.DomainTemplate,
			fmt.Sprintf("{%d...%d}", pool.ServerCountBegin, pool.ServerCountEnd),
		),
		poolTenant.ApiPort,
	)
	mounts := fmt.Sprintf(
		pool.MountPathTemplate,
		fmt.Sprintf("{1...%d}", pool.MountCount),
	)

	res := fmt.Sprintf("%s%s", urls, mounts)

	if poolTenant.DataPath != "" {
		res = joinPoolDir(res, poolTenant.DataPath)
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

func (pools *MinioServerPools) Stringify(tenant string) string {
	stringifiedPools := []string{}
	for _, pool := range *pools {
		stringifiedPools = append(stringifiedPools, pool.Stringify(tenant))
	}

	return strings.Join(stringifiedPools, " ")
}