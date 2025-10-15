//go:build linux

package runtime

import (
	"context"
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
)

// GetCPUQuotaToCPUCntByPidForCgroups1 converts the CPU quota applied to the calling process
// to a valid CPU cnt value. The quota is converted from float to int using round.
// If round == nil, DefaultRoundFunc is used.
// Only support cgroups1!
// Returns: cpuCount, quotaRatio (actual quota / cpuCount), status, error
func GetCPUQuotaToCPUCntByPidForCgroups1(
	ctx context.Context,
	actualCGRoot string,
	pid string,
	minValue int,
	round func(v float64) int,
) (int, float64, CPUQuotaStatus, error) {
	if round == nil {
		round = DefaultRoundFunc
	}

	cg, err := cgroups.NewCGroups(fmt.Sprintf("/proc/%s/mountinfo", pid), fmt.Sprintf("/proc/%s/cgroup", pid), actualCGRoot)
	if err != nil {
		log.Errorf(ctx, "get cgroup failed for cpu cnt, err: %v, pid: %v", err, pid)
		return -1, 1.0, CPUQuotaUndefined, err
	}
	quota, defined, err := cg.CPUQuota()
	if err != nil {
		log.Errorf(ctx, "get cgroup cpu quota failed, err: %v, pid: %v", err, pid)
	}
	if !defined {
		log.Warnf(ctx, "cpu quota is not defined, pid: %v", pid)
		return -1, 1.0, CPUQuotaUndefined, err
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

	log.Infof(ctx, "get cpu quota success, pid: %v, quota: %v, round quota: %v, ratio: %v", pid, quota, maxProcs, quotaRatio)
	if minValue > 0 && maxProcs < minValue {
		// When using minValue, recalculate ratio
		quotaRatio = quota / float64(minValue)
		return minValue, quotaRatio, CPUQuotaMinUsed, nil
	}
	return maxProcs, quotaRatio, CPUQuotaUsed, nil
}
