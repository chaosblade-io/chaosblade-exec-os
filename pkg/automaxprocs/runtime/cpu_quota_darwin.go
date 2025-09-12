//go:build darwin

package runtime

import "context"

// GetCPUQuotaToCPUCntByPidForCgroups2 converts the CPU quota applied to the calling process
// to a valid CPU cnt value for cgroup v2. On Darwin, cgroups are not available,
// so this function returns an error.
func GetCPUQuotaToCPUCntByPidForCgroups2(
	ctx context.Context,
	actualCGRoot string,
	pid string,
	minValue int,
	round func(v float64) int,
) (int, CPUQuotaStatus, error) {
	// On Darwin, cgroups are not available, so we return an error
	return -1, CPUQuotaUndefined, nil
}
