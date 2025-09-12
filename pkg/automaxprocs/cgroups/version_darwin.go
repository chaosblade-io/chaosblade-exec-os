//go:build darwin

package cgroups

import "context"

const (
	// CGroupV1FS is the cgroup v1 filesystem type
	CGroupV1FS = "cgroup"
	// CGroupV2FS is the cgroup v2 filesystem type
	CGroupV2FS = "cgroup2"
	// CGroupV2UnifiedMount is the unified mount point for cgroup v2
	CGroupV2UnifiedMount = "/sys/fs/cgroup"
)

// CGroupVersion represents the cgroup version
type CGroupVersion int

const (
	// CGroupV1 represents cgroup v1
	CGroupV1 CGroupVersion = iota
	// CGroupV2 represents cgroup v2
	CGroupV2
	// CGroupUnknown represents unknown cgroup version
	CGroupUnknown
)

// DetectCGroupVersion detects the cgroup version by checking the mount points
// On Darwin, cgroups are not available, so we return CGroupUnknown
func DetectCGroupVersion(ctx context.Context, cgroupRoot string) CGroupVersion {
	return CGroupUnknown
}

// IsCGroupV2 checks if the system is using cgroup v2
// On Darwin, cgroups are not available, so we return false
func IsCGroupV2(ctx context.Context, cgroupRoot string) bool {
	return false
}

// CGroupV2Control represents a cgroup v2 control group
type CGroupV2Control struct {
	path string
}

// NewCGroupV2Control creates a new CGroupV2Control instance
func NewCGroupV2Control(path string) *CGroupV2Control {
	return &CGroupV2Control{path: path}
}

// Path returns the path of the CGroupV2Control
func (cg *CGroupV2Control) Path() string {
	return cg.path
}
