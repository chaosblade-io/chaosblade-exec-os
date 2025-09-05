//go:build linux

package cgroups

import (
	"context"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
)

const (
	// _cgroupFSType is the Linux CGroup file system type used in
	// `/proc/$PID/mountinfo`.
	_cgroupFSType = "cgroup"
	// _cgroupSubsysCPU is the CPU CGroup subsystem.
	_cgroupSubsysCPU = "cpu"
	// _cgroupCPUCFSQuotaUsParam is the file name for the CGroup CFS quota
	// parameter.
	_cgroupCPUCFSQuotaUsParam = "cpu.cfs_quota_us"
	// _cgroupCPUCFSPeriodUsParam is the file name for the CGroup CFS period
	// parameter.
	_cgroupCPUCFSPeriodUsParam = "cpu.cfs_period_us"
)

// CGroups is a map that associates each CGroup with its subsystem name.
type CGroups map[string]*CGroup

// NewCGroups returns a new *CGroups from given `mountinfo` and `cgroup` files
// under for some process under `/proc` file system (see also proc(5) for more
// information).
func NewCGroups(procPathMountInfo, procPathCGroup, actualCGRoot string) (CGroups, error) {
	cgroupSubsystems, err := parseCGroupSubsystems(procPathCGroup)
	if err != nil {
		return nil, err
	}

	cgroups := make(CGroups)
	newMountPoint := func(mp *MountPoint) error {
		if mp.FSType != _cgroupFSType {
			return nil
		}

		for _, opt := range mp.SuperOptions {
			subsys, exists := cgroupSubsystems[opt]
			if !exists {
				continue
			}

			cgroupPath, err := mp.CustomTranslate(subsys.Name)
			if err != nil {
				return err
			}
			log.Infof(context.TODO(), "cgroup, opt: %v, path: %v", opt, cgroupPath)
			cgroups[opt] = NewCGroup(cgroupPath)
		}

		return nil
	}

	if err := parseMountInfo(procPathMountInfo, actualCGRoot, newMountPoint); err != nil {
		return nil, err
	}
	return cgroups, nil
}

// CPUQuota returns the CPU quota applied with the CPU cgroup controller.
// It is a result of `cpu.cfs_quota_us / cpu.cfs_period_us`. If the value of
// `cpu.cfs_quota_us` was not set (-1), the method returns `(-1, nil)`.
func (cg CGroups) CPUQuota() (float64, bool, error) {
	cpuCGroup, exists := cg[_cgroupSubsysCPU]
	if !exists {
		log.Warnf(context.Background(), "CPU cgroup is not found in cg")
		return -1, false, nil
	}

	cfsQuotaUs, err := cpuCGroup.readInt(_cgroupCPUCFSQuotaUsParam)
	if defined := cfsQuotaUs > 0; err != nil || !defined {
		log.Warnf(context.Background(), "CPU cgroup quota is not defined, err: %v", err)
		return -1, defined, err
	}

	cfsPeriodUs, err := cpuCGroup.readInt(_cgroupCPUCFSPeriodUsParam)
	if defined := cfsPeriodUs > 0; err != nil || !defined {
		log.Warnf(context.Background(), "CPU cgroup period is not defined, err: %v", err)
		return -1, defined, err
	}
	log.Infof(context.Background(), "CPU cgroup quota: %v, period: %v", cfsQuotaUs, cfsPeriodUs)
	return float64(cfsQuotaUs) / float64(cfsPeriodUs), true, nil
}
