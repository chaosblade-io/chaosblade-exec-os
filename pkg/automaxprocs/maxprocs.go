package automaxprocs

import (
	"context"
	"runtime"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
	iruntime "github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/runtime"
)

// GetCPUCntByPidForCgroups1 actualCGRoot 用于调整 mountinfo 下的挂载点 cgroup 路径
// 返回: cpuCount, quotaRatio (实际quota/cpuCount的比值), error
func GetCPUCntByPidForCgroups1(ctx context.Context, actualCGRoot, pid string) (int, float64, error) {
	cnt, ratio, status, err := iruntime.GetCPUQuotaToCPUCntByPidFroCgroups1(
		ctx,
		actualCGRoot,
		pid,
		1,
		iruntime.DefaultRoundFunc,
	)
	numCPU := runtime.NumCPU()
	if err != nil {
		log.Errorf(ctx, "error on GetCPUQuotaToCPUCntByPidFroCgroups1, err: %v, use NumCPU instead", err)
		return numCPU, 1.0, err
	}

	switch status {
	case iruntime.CPUQuotaUndefined:
		log.Warnf(ctx, "maxprocs: Leaving NumCPU=%v: CPU quota undefined", numCPU)
		return numCPU, 1.0, nil
	case iruntime.CPUQuotaMinUsed:
		log.Warnf(ctx, "CPU quota below minimum: %v, ratio: %v", cnt, ratio)
	case iruntime.CPUQuotaUsed:
		log.Infof(ctx, "get numCPU count by pid %s, cgroups1 cpu quota: %d, ratio: %v, numCPU: %v", pid, cnt, ratio, numCPU)
	}

	return cnt, ratio, nil
}

// GetCPUCntByPidForCgroups2 支持 cgroup v2 的 CPU 数量获取
// 返回: cpuCount, quotaRatio (实际quota/cpuCount的比值), error
func GetCPUCntByPidForCgroups2(ctx context.Context, actualCGRoot, pid string) (int, float64, error) {
	cnt, ratio, status, err := iruntime.GetCPUQuotaToCPUCntByPidForCgroups2(
		ctx,
		actualCGRoot,
		pid,
		1,
		iruntime.DefaultRoundFunc,
	)
	numCPU := runtime.NumCPU()
	if err != nil {
		log.Errorf(ctx, "error on GetCPUQuotaToCPUCntByPidForCgroups2, err: %v, use NumCPU instead", err)
		return numCPU, 1.0, err
	}

	switch status {
	case iruntime.CPUQuotaUndefined:
		log.Warnf(ctx, "maxprocs: Leaving NumCPU=%v: CPU quota undefined", numCPU)
		return numCPU, 1.0, nil
	case iruntime.CPUQuotaMinUsed:
		log.Warnf(ctx, "CPU quota below minimum: %v, ratio: %v", cnt, ratio)
	case iruntime.CPUQuotaUsed:
		log.Infof(ctx, "get numCPU count by pid %s, cgroups2 cpu quota: %d, ratio: %v, numCPU: %v", pid, cnt, ratio, numCPU)
	}

	return cnt, ratio, nil
}

// GetCPUCntByPid 自动检测 cgroup 版本并获取 CPU 数量
// 返回: cpuCount, quotaRatio (实际quota/cpuCount的比值), error
// quotaRatio 用于调整用户传入的百分比，例如: quota=0.6核, cpuCount=1, ratio=0.6
// 用户传入80%负载时，实际应该是 80% * 0.6 = 48% 的单核负载
func GetCPUCntByPid(ctx context.Context, actualCGRoot, pid string) (int, float64, error) {
	// 检测 cgroup 版本
	version := cgroups.DetectCGroupVersion(ctx, actualCGRoot)

	switch version {
	case cgroups.CGroupV2:
		log.Infof(ctx, "detected cgroup v2, using v2 implementation")
		return GetCPUCntByPidForCgroups2(ctx, actualCGRoot, pid)
	case cgroups.CGroupV1:
		log.Infof(ctx, "detected cgroup v1, using v1 implementation")
		return GetCPUCntByPidForCgroups1(ctx, actualCGRoot, pid)
	case cgroups.CGroupUnknown:
		log.Warnf(ctx, "cgroup not available (e.g., on Darwin), using runtime.NumCPU()")
		return runtime.NumCPU(), 1.0, nil
	default:
		log.Warnf(ctx, "unknown cgroup version, falling back to v1 implementation")
		return GetCPUCntByPidForCgroups1(ctx, actualCGRoot, pid)
	}
}
