# About

Ferio ensures that the cluster is synchronized with the following evens:
- Binary Updates
- Server Pools Additions

The following assumption is made: Changes (binary updates or server pool additions) are not made until all previous changes have already been processed by the cluster

# Workflow

## Boot

When a ferio boot, if a minio service file is absent:
- Get the binary release info + download the binary
- Get the server pools info
- Generate the minio service file
- Check if there is a server pools synchronization in progress and if so, synchronize on server pools change + start minio
- Check if there is a binary update in progress and if so, synchronize on binary update + start minio
- Make sure minio is started
- Follow runtime procedure

When a ferio boot, if a minio service file is present:
- Get the server pools info
- Get the binary release info
- Check if there is a server pools synchronization in progress and if so, synchronize on pool change + start minio
- Check if there is a binary update in progress and if so, synchronize on binary update + start minio
- Follow runtime procedure

Follow runtime procedure

## Runtime

When a node runs, it will:
- Listen on server pools change if updated: Synchronize on server pools change + start minio
- Listen on binary update and if updated: Synchronize on binary update + start minio

# Synchronization tasks

## Pool Change

1. Synchronize Acknowledgment
2. Synchronize Minio Shutdown
3. Synchronize Systemd Service Update

## Binary Update

1. Synchronize Binary Download
2. Synchronize Minio Shutdown
3. Synchronize Systemd Service Update