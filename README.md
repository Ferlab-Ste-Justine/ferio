# About

Ferio is a tool that allows simultaneous updates of a distributed minio cluster managed by systemd using an etcd store to specify configuration and for synchronization.

The parameters currently specifiable are:
  - The minio binary to use
  - The volume pools (currently, only adding additional volume pools is supported)

Further capabilities will be added as the need arises.

The tool has the following 2 priorities (by order of importance):
  - Ensuring that all running minio instances are homogeneous
  - Minimize downtime

To optimize on the second priority, the ferio processes across the minio nodes will synchronize until all the nodes are ready to change the systemd configuration and restart minio. Only then will the minio processes by restarted. This should lead to a downtime that is very close to the time minio takes to shutdown and startup.

# Configuration

...

# Etcd Keyspace

...