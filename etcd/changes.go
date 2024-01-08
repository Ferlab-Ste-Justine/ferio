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

func HandleChanges(cli *client.EtcdClient, prefix string, startPools *MinioServerPools, poolsAction ServerPoolsChangeAction, startRel *MinioRelease, relAction ReleaseChangeAction, log logger.Logger) <-chan error {
	errCh := make(chan error)
	go func() {
		defer close(errCh)

		log.Infof("[etcd] Starting to watch for minio release and server pool changes")
	
		relConfigKey := fmt.Sprintf(ETCD_RELEASE_CONFIG_KEY, prefix)
		poolsConfigKey := fmt.Sprintf(ETCD_POOLS_CONFIG_KEY, prefix)

		rel, relRev, getRelErr := GetMinioRelease(cli, prefix)
		if getRelErr != nil {
			errCh <- getRelErr
			return
		}

		pools, poolsRev, getPoolsErr := GetMinioServerPools(cli, prefix)
		if getPoolsErr != nil {
			errCh <- getPoolsErr
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

			pools, poolsRev, getPoolsErr = GetMinioServerPools(cli, prefix)
			if getPoolsErr != nil {
				errCh <- getPoolsErr
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

			rel, relRev, getRelErr = GetMinioRelease(cli, prefix)
			if getRelErr != nil {
				errCh <- getRelErr
				return
			}
		}

		wcPoolsCh := cli.Watch(poolsConfigKey, client.WatchOptions{
			Revision: poolsRev + 1,
			IsPrefix: false,
			TrimPrefix: false,
		})

		wcRelCh := cli.Watch(relConfigKey, client.WatchOptions{
			Revision: relRev + 1,
			IsPrefix: false,
			TrimPrefix: false,
		})

		for true {
			select {
			case poolsInfo := <-wcPoolsCh:
				pools := MinioServerPools{}

				if poolsInfo.Error != nil {
					errCh <- poolsInfo.Error
					return
				}
	
				if len(poolsInfo.Changes.Deletions) > 0 {
					errCh <- errors.New("Got an unexpected etcd key deletion while looking for server pools changes")
					return
				}
	
				err := yaml.Unmarshal([]byte(poolsInfo.Changes.Upserts[poolsConfigKey].Value), &pools)
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
			case relInfo := <-wcRelCh:
				rel := MinioRelease{}

				if relInfo.Error != nil {
					errCh <- relInfo.Error
					return
				}
	
				if len(relInfo.Changes.Deletions) > 0 {
					errCh <- errors.New("Got an unexpected etcd key deletion while looking for release changes")
					return
				}
	
				err := yaml.Unmarshal([]byte(relInfo.Changes.Upserts[relConfigKey].Value), &rel)
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