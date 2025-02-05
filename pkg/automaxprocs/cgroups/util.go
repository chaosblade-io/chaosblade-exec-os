package cgroups

import (
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const hostDefaultCgroupFsPath = "/sys/fs/cgroup/"

func replaceCgroupFsPathForDaemonSetPod(mountPointPath, actualCGRoot string) string {
	if len(actualCGRoot) == 0 {
		actualCGRoot = spec.DefaultCGroupPath
	}
	return strings.Replace(mountPointPath, hostDefaultCgroupFsPath, actualCGRoot, 1)
}
