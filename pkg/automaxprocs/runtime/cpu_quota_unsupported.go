//go:build !linux

package runtime

import (
	"context"
)

func GetCPUQuotaToCPUCntByPidFroCgroups1(_ context.Context, _, _ string, _ int, _ func(v float64) int) (int, CPUQuotaStatus, error) {
	return -1, CPUQuotaUndefined, nil
}

// CPUQuotaToGOMAXPROCS converts the CPU quota applied to the calling process
// to a valid GOMAXPROCS value. This is Linux-specific and not supported in the
// current OS.
func CPUQuotaToGOMAXPROCS(_ int, _ func(v float64) int) (int, CPUQuotaStatus, error) {
	return -1, CPUQuotaUndefined, nil
}
