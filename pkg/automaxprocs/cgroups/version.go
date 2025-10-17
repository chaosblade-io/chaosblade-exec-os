//go:build linux

package cgroups

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
)

const (
	// CGroupV1FS is the cgroup v1 filesystem type
	CGroupV1FS = "cgroup"
	// CGroupV2FS is the cgroup v2 filesystem type
	CGroupV2FS = "cgroup2"
	// CGroupV2UnifiedMount is the unified mount point for cgroup v2
	CGroupV2UnifiedMount = "/sys/fs/cgroup"
)

// CGroupVersion represents the cgroup version
type CGroupVersion int

const (
	// CGroupV1 represents cgroup v1
	CGroupV1 CGroupVersion = iota
	// CGroupV2 represents cgroup v2
	CGroupV2
	// CGroupUnknown represents unknown cgroup version
	CGroupUnknown
)

// DetectCGroupVersion detects the cgroup version by checking the mount points
func DetectCGroupVersion(ctx context.Context, cgroupRoot string) CGroupVersion {
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}

	// Check if cgroup v2 unified mount exists
	unifiedMount := filepath.Join(cgroupRoot, "cgroup.controllers")
	if _, err := os.Stat(unifiedMount); err == nil {
		log.Infof(ctx, "detected cgroup v2 unified mount at %s", cgroupRoot)
		return CGroupV2
	}

	// Check mountinfo for cgroup v2
	mountInfoPath := "/proc/self/mountinfo"
	if file, err := os.Open(mountInfoPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				mountPoint := fields[4]
				fstype := fields[6]
				// Check if it's a cgroup v2 mount
				if fstype == CGroupV2FS && strings.HasPrefix(mountPoint, cgroupRoot) {
					log.Infof(ctx, "detected cgroup v2 mount at %s", mountPoint)
					return CGroupV2
				}
			}
		}
	}

	// Check for cgroup v1 mounts
	if file, err := os.Open(mountInfoPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				fstype := fields[6]
				if fstype == CGroupV1FS {
					log.Infof(ctx, "detected cgroup v1 mount")
					return CGroupV1
				}
			}
		}
	}

	log.Warnf(ctx, "unable to detect cgroup version, defaulting to v1")
	return CGroupV1
}

// IsCGroupV2 checks if the system is using cgroup v2
func IsCGroupV2(ctx context.Context, cgroupRoot string) bool {
	return DetectCGroupVersion(ctx, cgroupRoot) == CGroupV2
}

// CGroupV2Control represents a cgroup v2 control group
type CGroupV2Control struct {
	path string
}

// NewCGroupV2Control creates a new CGroupV2Control instance
func NewCGroupV2Control(path string) *CGroupV2Control {
	return &CGroupV2Control{path: path}
}

// Path returns the path of the CGroupV2Control
func (cg *CGroupV2Control) Path() string {
	return cg.path
}
