package automaxprocs

import (
	"context"
	"runtime"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
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

// GetCPUCntByPidForCgroups2 支持 cgroup v2 的 CPU 数量获取
func GetCPUCntByPidForCgroups2(ctx context.Context, actualCGRoot, pid string) (int, error) {
	cnt, status, err := iruntime.GetCPUQuotaToCPUCntByPidForCgroups2(
		ctx,
		actualCGRoot,
		pid,
		1,
		iruntime.DefaultRoundFunc,
	)
	numCPU := runtime.NumCPU()
	if err != nil {
		log.Errorf(ctx, "error on GetCPUQuotaToCPUCntByPidForCgroups2, err: %v, use NumCPU instead", err)
		return numCPU, err
	}

	switch status {
	case iruntime.CPUQuotaUndefined:
		log.Warnf(ctx, "maxprocs: Leaving NumCPU=%v: CPU quota undefined", numCPU)
		return numCPU, nil
	case iruntime.CPUQuotaMinUsed:
		log.Warnf(ctx, "CPU quota below minimum: %v", cnt)
	case iruntime.CPUQuotaUsed:
		log.Infof(ctx, "get numCPU count by pid %s, cgroups2 cpu quota: %d, numCPU: %v", pid, cnt, numCPU)
	}

	return cnt, nil
}

// GetCPUCntByPid 自动检测 cgroup 版本并获取 CPU 数量
func GetCPUCntByPid(ctx context.Context, actualCGRoot, pid string) (int, error) {
	// 检测 cgroup 版本
	version := cgroups.DetectCGroupVersion(ctx, actualCGRoot)

	switch version {
	case cgroups.CGroupV2:
		log.Infof(ctx, "detected cgroup v2, using v2 implementation")
		return GetCPUCntByPidForCgroups2(ctx, actualCGRoot, pid)
	case cgroups.CGroupV1:
		log.Infof(ctx, "detected cgroup v1, using v1 implementation")
		return GetCPUCntByPidForCgroups1(ctx, actualCGRoot, pid)
	default:
		log.Warnf(ctx, "unknown cgroup version, falling back to v1 implementation")
		return GetCPUCntByPidForCgroups1(ctx, actualCGRoot, pid)
	}
}
