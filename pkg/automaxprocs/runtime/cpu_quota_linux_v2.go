//go:build linux

package runtime

import (
	"context"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
)

// GetCPUQuotaToCPUCntByPidForCgroups2 converts the CPU quota applied to the calling process
// to a valid CPU cnt value for cgroup v2. The quota is converted from float to int using round.
// If round == nil, DefaultRoundFunc is used.
// Returns: cpuCount, quotaRatio (actual quota / cpuCount), status, error
func GetCPUQuotaToCPUCntByPidForCgroups2(
	ctx context.Context,
	actualCGRoot string,
	pid string,
	minValue int,
	round func(v float64) int,
) (int, float64, CPUQuotaStatus, error) {
	if round == nil {
		round = DefaultRoundFunc
	}

	// Find the cgroup v2 path for the given PID
	cgroupPath, err := cgroups.FindCGroupV2Path(ctx, pid, actualCGRoot)
	if err != nil {
		log.Errorf(ctx, "failed to find cgroup v2 path for PID %s: %v", pid, err)
		return -1, 1.0, CPUQuotaUndefined, err
	}

	if cgroupPath == "" {
		log.Warnf(ctx, "no cgroup v2 path found for PID %s", pid)
		return -1, 1.0, CPUQuotaUndefined, nil
	}

	// Create CGroupV2Impl instance and get CPU quota
	cg := cgroups.NewCGroupV2Impl(cgroupPath)
	quota, defined, err := cg.CPUQuota()
	if err != nil {
		log.Errorf(ctx, "failed to get cgroup v2 cpu quota for PID %s: %v", pid, err)
		return -1, 1.0, CPUQuotaUndefined, err
	}

	if !defined {
		log.Warnf(ctx, "cpu quota is not defined for PID %s in cgroup v2", pid)
		return -1, 1.0, CPUQuotaUndefined, nil
	}

	maxProcs := round(quota)
	// Calculate the ratio: actual quota / rounded cpuCount
	// Example: quota=0.6, maxProcs=1, ratio=0.6
	var quotaRatio float64
	if maxProcs > 0 {
		quotaRatio = quota / float64(maxProcs)
	} else {
		quotaRatio = 1.0
	}

	log.Infof(ctx, "get cgroup v2 cpu quota success, pid: %v, quota: %v, round quota: %v, ratio: %v", pid, quota, maxProcs, quotaRatio)
	if minValue > 0 && maxProcs < minValue {
		// When using minValue, recalculate ratio
		quotaRatio = quota / float64(minValue)
		return minValue, quotaRatio, CPUQuotaMinUsed, nil
	}
	return maxProcs, quotaRatio, CPUQuotaUsed, nil
}
