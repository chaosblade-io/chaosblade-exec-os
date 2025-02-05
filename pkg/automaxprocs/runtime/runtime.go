package runtime

import "math"

// CPUQuotaStatus presents the status of how CPU quota is used
type CPUQuotaStatus int

const (
	// CPUQuotaUndefined is returned when CPU quota is undefined
	CPUQuotaUndefined CPUQuotaStatus = iota
	// CPUQuotaUsed is returned when a valid CPU quota can be used
	CPUQuotaUsed
	// CPUQuotaMinUsed is returned when CPU quota is smaller than the min value
	CPUQuotaMinUsed
)

// DefaultRoundFunc is the default function to convert CPU quota from float to int. It rounds the value down (floor).
func DefaultRoundFunc(v float64) int {
	return int(math.Ceil(v))
}
