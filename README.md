# About

Ferio is a tool that allows simultaneous updates of a distributed minio cluster managed by systemd using an etcd store to specify configuration and for synchronization.

The parameters currently specifiable are:
  - The minio binary to use
  - The volume pools (currently, only adding additional volume pools is supported)

Further capabilities will be added as the need arises.

# Behavior to Note

## Priorities

Ferio has the following 2 priorities (by order of importance):
  - Ensuring that all running minio instances are homogeneous
  - Minimize downtime

To ensure on the first priority, the minio services are all stopped and disabled before any changes are made to the deployment and minio binary downloads are done in a separate path from the currently running binary.

To optimize on the second priority after ensuring the first, the ferio processes across the minio nodes will synchronize until all the nodes are ready to change the systemd configuration and restart minio. Only then will the minio processes by restarted. This should lead to a downtime that is very close to the time minio takes to shutdown and startup.

Also note ferio will not interfere with the minio service outside of update operations. If your configuration etcd cluster becomes unavailable for some reason and your minio cluster is not updating, the ferio service will crash, but the minio service will run fine. Also more generally, the minio service won't be impacted by ferio restarting outside of update windows.

## Failing Fast

Ferio adopts a fail fast approach. It will retry on failing etcd queries before giving up, but it will not try to recover from other types of errors. It is expected that ferio will be managed by a scheduler like systemd that will reboot it on failure.

# Limitations

Beyond the update to the release and server pools occuring when the cluster first boots, concurrent updates are currently not supported and will possibly lead to deadlocks. Ensure that previous updates have completed across the cluster before updating the configuration further.

Info level logs will make the update status of various ferio instances very clear. Basic alert-oriented prometheus metrics are likely to be added in the future as well.

# Expectations

Ferio also expects a user and group called **minio** to exist on the system. All minio serviced managed by ferio will run under this user.

Ferio also expects most minio settings for the services it manages to be pre-specified in environment files. See the **minio_services** part of the configuration for details.

# Etcd Keyspace

Given an etcd key prefix of `/myconfprefix/`, the minio configuration in the etcd store is expected to have two keys with pre-determined suffixes.

Ferio will read and react to changes on the two keys listed below. Any other keys in the prefix will be ignored.

## Release

**key**: /myconfprefix/release

**Fields**:
  - **version**: Version of the minio binary. Should be a strictly increasing string like the yyyy-mm-dd date format for example.
  - **url**: Url where the minio binary can be downloaded
  - **checksum**: sha256 checksum of the minio binary to download

## Pools

**key**: /myconfprefix/pools

**Fields**:
  - **version**: Version of the configuration. Should be a strictly increasing string like the yyyy-mm-dd date format for example.
  - **pools**: Array of minio server pools, each entry contains the following fields...
    - **domain_template**: Domain template of the first server pool with a string place holder for the servers count range expansion. For example, an input of "server%s.minio.ferlab.lan" will be expanded by ferio to "server{<count begin>...<count end>}.minio.ferlab.lan"
    - **server_count_begin**: Should be an integer marking the domain of the first server in the first pool.
    - **server_count_end**: Should be an integer marking the domain of the last server in the first pool
    - **mount_path_template**: Path template of the disk paths on each server in the first pool, with a string placeholder for the count expansion. For example, an input of "/opt/mnt/volume%s" will be expanded to "/opt/mnt/volume{1...<mount count>}"
    - **mount_count**: Number of mounted volumes on each server in the server in the first pool
    - **api_port**: Default api port the minio servers in the pool will expose. If no tenants are specified, this api port will be used in the pool's configuration.
    - **tenants**: Optional tenants list to configure pools for separate sets of minio servers sharing disks (by using separate paths in their filesystem). Each entry should have the following keys:
      - **name**: Name of the tenant
      - **api_port**: Api ports the servers on the pool will be exposing
      - **data_path**: Path of the data, relative to the mount point of the disks. This is the directory used by the tenant on each disk in the pool.

# Configuration

The ferio configuration is a yaml file whose path can be specified with the **FERIO_CONFIG_FILE** environment variable. It defaults to a file named **config.yml** in the running directory.

The configuration format is:

- **binaries_dir**: Directory where ferio will download minio binaries
- **host**: Unique host entry of the node ferio runs on. If empty, the os hostname will be used
- **log_level**: Cutoff level of logging to show. Can be debug, info, warning or error
- **minio_services**: Array on minio services to manage on each node. For a single tenant setup, there can be a single entry. Omitting this field will result in a single entry with the **name** of **minio.service**, **env_path** of **/etc/minio/env** and **tenant_name** being empty. This corresponds to how ferio behaved before multi-tenancy was introduced and should be compatible with older setups. Otherwise, each entry should have the following fields:
  - **name**: Name of the service's systemd unit. Note that if **.service** is not a suffix for the name, it ferio will append it to the inputed value.
  - **tenant_name**: Name of the service's tenant which will be matched with the identical `pools[..].tenants[..].name` value in the ferio pools etcd key to figure out how to configure the volume pools for the minio service.
  - **env_path**: Path to the file containing minio environment variables. The file should contain an environment variable called **MINIO_OPTS** that should contain all command line arguments to pass to the **minio server** command. The file should not contain the **MINIO_VOLUMES** environment variable as ferio will manage this variable itself based on the configuration it reads from etcd.
- **etcd**: Parameters for the etcd connection. It takes the parameters listed below...
  - **config_prefix**: Key prefix to use for the externally updated minio configuration
  - **workspace_prefix**: Key prefix to use as an internal workspace for update synchronization between ferio instances across nodes
  - **endpoints**: List (ie, yaml array) of etcd server endpoints. Each endpoint should have the format `<ip>:<port>`
  - **connection_timeout**: Etcd connection timeout as a valid golang duration string format
  - **request_timeout**: Request timeout as a valid golang duration string format
  - **retry_interval**: Interval between retries for failing requests, as a valid golang duration string format
  - **retries**: Number of times to retry failing requests before giving up
  - **auth**: Authentication parameters for the etcd servers. It takes the parameters listed below...
	- **ca_cert**: Path to a CA certificate that will authentify the etcd servers
    - **client_cert**: Path to a client certificate that will authentify ferio against the etcd servers if certificate authentication is used.
    - **client_key**: Path to a client private key that will authentify ferio against the etcd servers if certificat authentication is used.
    - **password_auth**: Path to a passworth auth file if password authentication is used. The file should be a yaml file with the **username** and **password** properties defined