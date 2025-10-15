//go:build !linux

package runtime

import (
	"context"
)

// GetCPUQuotaToCPUCntByPidForCgroups1 is not support for non-linux systems because cgroups is a Linux kernal feature.
func GetCPUQuotaToCPUCntByPidForCgroups1(
	_ context.Context,
	_, _ string,
	_ int,
	_ func(v float64) int,
) (int, float64, CPUQuotaStatus, error) {
	return -1, 1.0, CPUQuotaUndefined, nil
}

// GetCPUQuotaToCPUCntByPidForCgroups2 is not support for non-linux systems because cgroups is a Linux kernal feature.
func GetCPUQuotaToCPUCntByPidForCgroups2(
	_ context.Context,
	_, _ string,
	_ int,
	_ func(v float64) int,
) (int, float64, CPUQuotaStatus, error) {
	return -1, 1.0, CPUQuotaUndefined, nil
}
