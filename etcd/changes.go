package etcd

import (
	"errors"
	"fmt"
	yaml "gopkg.in/yaml.v2"

	"github.com/Ferlab-Ste-Justine/ferio/logger"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
)

type ReleaseChangeAction func(*MinioRelease, *MinioServerPools) error

type ServerPoolsChangeAction func(*MinioServerPools, *MinioRelease) error

func GetConfigs(cli *client.EtcdClient, prefix string) (*MinioServerPools, *MinioRelease, int64, error) {
	poolsKey := fmt.Sprintf(ETCD_POOLS_CONFIG_KEY, prefix)
	relKey := fmt.Sprintf(ETCD_RELEASE_CONFIG_KEY, prefix)

	var pools MinioServerPools
	var rel MinioRelease

	info, err := cli.GetPrefix(prefix)
	if err != nil {
		return nil, nil, -1, err
	}

	val, ok := info.Keys[poolsKey]
	if !ok {
		return nil, nil, -1, errors.New(fmt.Sprintf("Server pools configuration not found at key %s", poolsKey))
	}

	err = yaml.Unmarshal([]byte(val.Value), &pools)
	if err != nil {
		return nil, nil, -1, errors.New(fmt.Sprintf("Error parsing the server pools configuration: %s", err.Error()))
	}

	val, ok = info.Keys[relKey]
	if !ok {
		return nil, nil, -1, errors.New(fmt.Sprintf("Release configuration not found at key %s", relKey))
	}

	err = yaml.Unmarshal([]byte(val.Value), &rel)
	if err != nil {
		return nil, nil, -1, errors.New(fmt.Sprintf("Error parsing the release configuration: %s", err.Error()))
	}

	return &pools, &rel, info.Revision, nil
}

func HandleChanges(cli *client.EtcdClient, prefix string, startPools *MinioServerPools, poolsAction ServerPoolsChangeAction, startRel *MinioRelease, relAction ReleaseChangeAction, log logger.Logger) <-chan error {
	errCh := make(chan error)
	go func() {
		defer close(errCh)

		log.Infof("[etcd] Starting to watch for minio release and server pool changes")
	
		relConfigKey := fmt.Sprintf(ETCD_RELEASE_CONFIG_KEY, prefix)
		poolsConfigKey := fmt.Sprintf(ETCD_POOLS_CONFIG_KEY, prefix)

		pools, rel, rev, getErr := GetConfigs(cli, prefix)
		if getErr != nil {
			errCh <- getErr
			return
		}

		if (*pools).Version != (*startPools).Version {
			log.Infof("[etcd] Handling new server pools configuration at version %s", (*pools).Version)

			rel, _, getErr := GetMinioRelease(cli, prefix)
			if getErr != nil {
				errCh <- getErr
				return
			}

			actErr := poolsAction(pools, rel)
			if actErr != nil {
				errCh <- actErr
				return
			}
		}

		if (*rel).Version != (*startRel).Version {
			log.Infof("[etcd] Handling new minio release at version %s", (*rel).Version)
			
			pools, _, getErr := GetMinioServerPools(cli, prefix)
			if getErr != nil {
				errCh <- getErr
				close(errCh)
				return
			}

			actErr := relAction(rel, pools)
			if actErr != nil {
				errCh <- actErr
				return
			}
		}

		wcCh := cli.Watch(prefix, client.WatchOptions{
			Revision: rev + 1,
			IsPrefix: true,
			TrimPrefix: false,
		})

		for info := range wcCh {
			if info.Error != nil {
				errCh <- info.Error
				return	
			}

			log.Debugf("[etcd] Detected a change in configurations keyspace")

			for _, val := range info.Changes.Deletions {
				if val == poolsConfigKey {
					errCh <- errors.New("Server pools configurations got deleted")
					return
				}
				
				if val == relConfigKey {
					errCh <- errors.New("Release configurations got deleted")
					return
				}
			}

			val, ok := info.Changes.Upserts[poolsConfigKey]
			if ok {
				pools := MinioServerPools{}

				err := yaml.Unmarshal([]byte(val.Value), &pools)
				if err != nil {
					errCh <- errors.New(fmt.Sprintf("Error parsing the server pools configuration: %s", err.Error()))
					return
				}

				log.Infof("[etcd] Handling new server pools configuration at version %s", pools.Version)
	
				rel, _, getErr := GetMinioRelease(cli, prefix)
				if getErr != nil {
					errCh <- getErr
					return
				}
	
				actErr := poolsAction(&pools, rel)
				if actErr != nil {
					errCh <- actErr
					return
				}
			}

			val, ok = info.Changes.Upserts[relConfigKey]
			if ok {
				rel := MinioRelease{}

				err := yaml.Unmarshal([]byte(val.Value), &rel)
				if err != nil {
					errCh <- errors.New(fmt.Sprintf("Error parsing the minio release configuration: %s", err.Error()))
					return
				}
	
				log.Infof("[etcd] Handling new minio release at version %s", rel.Version)
	
				pools, _, getErr := GetMinioServerPools(cli, prefix)
				if getErr != nil {
					errCh <- getErr
					return
				}
	
				actErr := relAction(&rel, pools)
				if actErr != nil {
					errCh <- actErr
					return
				}
			}
		}
	}()

	return errCh
}