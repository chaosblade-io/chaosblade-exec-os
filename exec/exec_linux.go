/*
 * Copyright 2025 The ChaosBlade Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package exec

import (
	"fmt"
	"os"

	"github.com/containerd/cgroups"
	"golang.org/x/sys/unix"
)

func PidPath(pid int) cgroups.Path {
	return pidPathFromFile(fmt.Sprintf("/proc/%d/cgroup", pid))
}

func pidPathFromFile(p string) cgroups.Path {
	paths, err := cgroups.ParseCgroupFile(p)
	if err != nil {
		return func(_ cgroups.Name) (string, error) {
			return "", fmt.Errorf("failed to parse cgroup file %s: %s", p, err.Error())
		}
	}

	return func(name cgroups.Name) (string, error) {
		root, ok := paths[string(name)]
		if !ok {
			if root, ok = paths["name="+string(name)]; !ok {
				// Load treats this sentinel as "controller inactive for this
				// PID" and skips it; an unrelated, unused controller (e.g. an
				// unmounted hugetlb) must not abort loading the controllers the
				// experiment actually needs.
				return "", cgroups.ErrControllerNotActive
			}
		}
		return root, nil
	}
}

func Hierarchy(root string) func() ([]cgroups.Subsystem, error) {
	return func() ([]cgroups.Subsystem, error) {
		subsystems, err := defaults(root)
		if err != nil {
			return nil, err
		}
		return mountedV1Subsystems(subsystems, unix.Statfs)
	}
}

// mountedV1Subsystems keeps only the controllers backed by a real cgroup v1
// mount. A plain Lstat is insufficient: on some cgroup v1 hosts the hugetlb
// controller directory exists under the cgroup tmpfs but is never mounted, so
// NewHugetlb succeeds while the controller is unusable. Enumerating it and
// then failing to resolve it for a PID (which has no hugetlb entry) made
// cgroups.Load return "controller is not supported", breaking otherwise
// healthy cpu/memory experiments.
func mountedV1Subsystems(subsystems []cgroups.Subsystem,
	statfs func(string, *unix.Statfs_t) error,
) ([]cgroups.Subsystem, error) {
	var enabled []cgroups.Subsystem
	for _, s := range pathers(subsystems) {
		controllerPath := s.Path("/")
		var fs unix.Statfs_t
		if err := statfs(controllerPath, &fs); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat cgroup controller %s at %s: %w", s.Name(), controllerPath, err)
		}
		// Statfs follows aliases such as cpu -> cpu,cpuacct.
		if fs.Type == unix.CGROUP_SUPER_MAGIC {
			enabled = append(enabled, s)
		}
	}
	return enabled, nil
}

// defaults returns all known groups
func defaults(root string) ([]cgroups.Subsystem, error) {
	h, err := cgroups.NewHugetlb(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	s := []cgroups.Subsystem{
		cgroups.NewNamed(root, "systemd"),
		cgroups.NewFreezer(root),
		cgroups.NewPids(root),
		cgroups.NewNetCls(root),
		cgroups.NewNetPrio(root),
		cgroups.NewPerfEvent(root),
		cgroups.NewCpuset(root),
		cgroups.NewCpu(root),
		cgroups.NewCpuacct(root),
		cgroups.NewMemory(root),
		cgroups.NewBlkio(root),
		cgroups.NewRdma(root),
	}
	// only add the devices cgroup if we are not in a user namespace
	// because modifications are not allowed
	if !cgroups.RunningInUserNS() {
		s = append(s, cgroups.NewDevices(root))
	}
	// add the hugetlb cgroup if error wasn't due to missing hugetlb
	// cgroup support on the host
	if err == nil {
		s = append(s, h)
	}
	return s, nil
}

type pather interface {
	cgroups.Subsystem
	Path(path string) string
}

func pathers(subystems []cgroups.Subsystem) []pather {
	var out []pather
	for _, s := range subystems {
		if p, ok := s.(pather); ok {
			out = append(out, p)
		}
	}
	return out
}
