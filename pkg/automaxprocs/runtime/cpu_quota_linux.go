//go:build linux

package runtime

import (
	"context"
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
)

// GetCPUQuotaToCPUCntByPidFroCgroups1 converts the CPU quota applied to the calling process
// to a valid CPU cnt value. The quota is converted from float to int using round.
// If round == nil, DefaultRoundFunc is used.
// Only support cgroups1!
func GetCPUQuotaToCPUCntByPidFroCgroups1(
	ctx context.Context,
	actualCGRoot string,
	pid string,
	minValue int,
	round func(v float64) int,
) (int, CPUQuotaStatus, error) {
	if round == nil {
		round = DefaultRoundFunc
	}

	cg, err := cgroups.NewCGroups(fmt.Sprintf("/proc/%s/mountinfo", pid), fmt.Sprintf("/proc/%s/cgroup", pid), actualCGRoot)
	if err != nil {
		log.Errorf(ctx, "get cgroup failed for cpu cnt, err: %v, pid: %v", err, pid)
		return -1, CPUQuotaUndefined, err
	}
	quota, defined, err := cg.CPUQuota()
	if err != nil {
		log.Errorf(ctx, "get cgroup cpu quota failed, err: %v, pid: %v", err, pid)
	}
	if !defined {
		log.Warnf(ctx, "cpu quota is not defined, pid: %v", pid)
		return -1, CPUQuotaUndefined, err
	}

	maxProcs := round(quota)
	log.Infof(ctx, "get cpu quota success, pid: %v, quota: %v, round quota: %v", pid, quota, maxProcs)
	if minValue > 0 && maxProcs < minValue {
		return minValue, CPUQuotaMinUsed, nil
	}
	return maxProcs, CPUQuotaUsed, nil
}
