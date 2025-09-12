//go:build linux

package cgroups

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
)

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
// Format: "quota period" or "max" for unlimited
func (cg *CGroupV2Impl) CPUQuota() (float64, bool, error) {
	quotaFile := filepath.Join(cg.path, CGroupV2CPUQuotaFile)
	content, err := os.ReadFile(quotaFile)
	if err != nil {
		log.Errorf(context.Background(), "failed to read cpu.max file: %v", err)
		return 0, false, err
	}

	quotaStr := strings.TrimSpace(string(content))
	log.Infof(context.Background(), "cgroup v2 cpu.max content: %s", quotaStr)

	// Parse "quota period" format
	parts := strings.Fields(quotaStr)
	if len(parts) != 2 {
		log.Errorf(context.Background(), "invalid cpu.max format: %s", quotaStr)
		return 0, false, nil
	}

	log.Debugf(context.Background(), "cgroup v2 cpu quota: %s", parts[0])
	// Handle "max" case for quota (unlimited)
	if parts[0] == "max" {
		log.Debugf(context.Background(), "cgroup v2 cpu quota is unlimited")
		return 0, false, nil
	}

	quota, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		log.Errorf(context.Background(), "failed to parse cpu quota: %v", err)
		return 0, false, err
	}

	period, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Errorf(context.Background(), "failed to parse cpu period: %v", err)
		return 0, false, err
	}

	if period <= 0 {
		log.Errorf(context.Background(), "invalid cpu period: %d", period)
		return 0, false, nil
	}

	cpuQuota := float64(quota) / float64(period)
	log.Infof(context.Background(), "cgroup v2 cpu quota: %f (quota: %d, period: %d)", cpuQuota, quota, period)
	return cpuQuota, true, nil
}

// MemoryLimit returns the memory limit for cgroup v2
func (cg *CGroupV2Impl) MemoryLimit() (int64, bool, error) {
	limitFile := filepath.Join(cg.path, CGroupV2MemoryLimitFile)
	content, err := os.ReadFile(limitFile)
	if err != nil {
		log.Errorf(context.Background(), "failed to read memory.max file: %v", err)
		return 0, false, err
	}

	limitStr := strings.TrimSpace(string(content))
	log.Infof(context.Background(), "cgroup v2 memory.max content: %s", limitStr)

	// Handle "max" case (unlimited)
	if limitStr == "max" {
		return 0, false, nil
	}

	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		log.Errorf(context.Background(), "failed to parse memory limit: %v", err)
		return 0, false, err
	}

	log.Infof(context.Background(), "cgroup v2 memory limit: %d", limit)
	return limit, true, nil
}

// FindCGroupV2Path finds the cgroup v2 path for a given PID
func FindCGroupV2Path(ctx context.Context, pid string, cgroupRoot string) (string, error) {
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}

	// Read /proc/PID/cgroup to find the cgroup path
	cgroupFile := filepath.Join("/proc", pid, "cgroup")
	content, err := os.ReadFile(cgroupFile)
	if err != nil {
		log.Errorf(ctx, "failed to read cgroup file for PID %s: %v", pid, err)
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// cgroup v2 format: 0::/path/to/cgroup
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" && parts[1] == "" {
			cgroupPath := parts[2]
			fullPath := filepath.Join(cgroupRoot, cgroupPath)
			log.Infof(ctx, "found cgroup v2 path for PID %s: %s", pid, fullPath)
			return fullPath, nil
		}
	}

	return "", nil
}
