//go:build !linux

package runtime

import (
	"context"
)

func GetCPUQuotaToCPUCntByPidForCgroups1(
	_ context.Context,
	_, _ string,
	_ int,
	_ func(v float64) int,
) (int, float64, CPUQuotaStatus, error) {
	return -1, 1.0, CPUQuotaUndefined, nil
}

func GetCPUQuotaToCPUCntByPidForCgroups2(
	_ context.Context,
	_, _ string,
	_ int,
	_ func(v float64) int,
) (int, float64, CPUQuotaStatus, error) {
	return -1, 1.0, CPUQuotaUndefined, nil
}
