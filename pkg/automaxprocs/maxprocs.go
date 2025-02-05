package automaxprocs

import (
	"context"
	"runtime"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	iruntime "github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/runtime"
)

// GetCPUCntByPidForCgroups1 actualCGRoot 用于调整 mountinfo 下的挂载点 cgroup 路径
func GetCPUCntByPidForCgroups1(ctx context.Context, actualCGRoot, pid string) (int, error) {
	cnt, status, err := iruntime.GetCPUQuotaToCPUCntByPidFroCgroups1(
		ctx,
		actualCGRoot,
		pid,
		1,
		iruntime.DefaultRoundFunc,
	)
	numCPU := runtime.NumCPU()
	if err != nil {
		log.Errorf(ctx, "error on GetCPUQuotaToCPUCntByPidFroCgroups1, err: %v, use NumCPU instead", err)
		return numCPU, err
	}

	switch status {
	case iruntime.CPUQuotaUndefined:
		log.Warnf(ctx, "maxprocs: Leaving NumCPU=%v: CPU quota undefined", numCPU)
		return numCPU, nil
	case iruntime.CPUQuotaMinUsed:
		log.Warnf(ctx, "CPU quota below minimum: %v", cnt)
	case iruntime.CPUQuotaUsed:
		log.Infof(ctx, "get numCPU count by pid %s, cgroups1 cpu quota: %d, numCPU: %v", pid, cnt, numCPU)
	}

	return cnt, nil
}
