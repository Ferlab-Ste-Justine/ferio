package etcd

const ETCD_RELEASES_CONFIG_PREFIX = "%sreleases/"

type MinioRelease struct {
	Version  string
	Url      string
	Checksum string
}