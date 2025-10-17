//go:build darwin

package cgroups

import "context"

const (
	// CGroupV2CPUController is the CPU controller for cgroup v2
	CGroupV2CPUController = "cpu"
	// CGroupV2CPUQuotaFile is the CPU quota file for cgroup v2
	CGroupV2CPUQuotaFile = "cpu.max"
	// CGroupV2MemoryController is the memory controller for cgroup v2
	CGroupV2MemoryController = "memory"
	// CGroupV2MemoryLimitFile is the memory limit file for cgroup v2
	CGroupV2MemoryLimitFile = "memory.max"
)

// CGroupV2Impl represents a cgroup v2 control group implementation
type CGroupV2Impl struct {
	path string
}

// NewCGroupV2Impl creates a new CGroupV2Impl instance
func NewCGroupV2Impl(path string) *CGroupV2Impl {
	return &CGroupV2Impl{path: path}
}

// CPUQuota returns the CPU quota for cgroup v2
// On Darwin, cgroups are not available, so this function returns an error
func (cg *CGroupV2Impl) CPUQuota() (float64, bool, error) {
	return 0, false, nil
}

// MemoryLimit returns the memory limit for cgroup v2
// On Darwin, cgroups are not available, so this function returns an error
func (cg *CGroupV2Impl) MemoryLimit() (int64, bool, error) {
	return 0, false, nil
}

// FindCGroupV2Path finds the cgroup v2 path for a given PID
// On Darwin, cgroups are not available, so this function returns an error
func FindCGroupV2Path(ctx context.Context, pid string, cgroupRoot string) (string, error) {
	return "", nil
}
