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

// Coverage:
//  1. Mounted v1 controllers are kept; an unmounted hugetlb directory (empty
//     tmpfs entry), a missing directory and a v2 mount are excluded.
//  2. Symlink aliases such as cpu -> cpu,cpuacct are identified by the target
//     filesystem type.
//  3. Permission errors are returned instead of being silently treated as an
//     unsupported controller.
//  4. Combined controllers and named systemd entries of a target PID are
//     resolved; missing controllers return a skippable sentinel while broken
//     or vanished proc files keep their real error.
//  5. The real cgroups.Load skips an optional controller absent from the
//     target PID and still loads memory, and fails only when no usable
//     controller remains.
//  6. mem requires memory and cpu requires both cpu/cpuacct; a required
//     controller missing from the mount list, proc file or target directory
//     must fail the experiment via the final load result.
//  7. An optional missing hugetlb controller must not block other experiments,
//     while non-resource targets do not gain cpu/memory requirements.
//  8. Nil statistics or nil usage fields fail; legitimate zero values pass.
//  9. Statistics read errors (vanished files, permission errors) propagate
//     instead of becoming zero usage; verified with the real readers.
//
// Approach: mount types are injected so no host mount changes are needed.
// Proc files and group directories use temp dirs, while loading goes through
// the project-pinned real containerd cgroups library.
package exec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containerd/cgroups"
	v1 "github.com/containerd/cgroups/stats/v1"
	"golang.org/x/sys/unix"
)

func TestMountedV1Subsystems(t *testing.T) {
	root := t.TempDir()
	subsystems := []cgroups.Subsystem{
		cgroups.NewMemory(root), cgroups.NewNamed(root, "hugetlb"),
		cgroups.NewCpu(root), cgroups.NewPids(root),
	}
	// memory is a real v1 mount; hugetlb is an empty tmpfs directory without a
	// mount; the cpu path is missing; pids points to a v2 filesystem and must
	// not be handed to the v1 loader.
	types := map[string]int64{
		filepath.Join(root, "memory"):  unix.CGROUP_SUPER_MAGIC,
		filepath.Join(root, "hugetlb"): unix.TMPFS_MAGIC,
		filepath.Join(root, "pids"):    unix.CGROUP2_SUPER_MAGIC,
	}
	got, err := mountedV1Subsystems(subsystems, func(p string, fs *unix.Statfs_t) error {
		kind, ok := types[p]
		if !ok {
			return os.ErrNotExist
		}
		fs.Type = kind
		return nil
	})
	if err != nil || len(got) != 1 || got[0].Name() != cgroups.Memory {
		t.Fatalf("expected only memory, got %v, error %v", got, err)
	}
}

func TestMountedV1SubsystemsRealEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "hugetlb"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Using the real Statfs proves that mere directory existence is not enough.
	got, err := mountedV1Subsystems([]cgroups.Subsystem{cgroups.NewNamed(root, "hugetlb")}, unix.Statfs)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty directory accepted: %v, %v", got, err)
	}
}

func TestMountedV1SubsystemsAlias(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "cpu,cpuacct")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("cpu,cpuacct", filepath.Join(root, "cpu")); err != nil {
		t.Fatal(err)
	}
	got, err := mountedV1Subsystems([]cgroups.Subsystem{cgroups.NewCpu(root)},
		func(p string, fs *unix.Statfs_t) error {
			resolved, err := filepath.EvalSymlinks(p)
			if err != nil {
				return err
			}
			if resolved != target {
				t.Fatalf("wrong alias target: %s", resolved)
			}
			fs.Type = unix.CGROUP_SUPER_MAGIC
			return nil
		})
	if err != nil || len(got) != 1 || got[0].Name() != cgroups.Cpu {
		t.Fatalf("cpu alias rejected: %v, %v", got, err)
	}
}

func TestMountedV1SubsystemsPermissionError(t *testing.T) {
	_, err := mountedV1Subsystems([]cgroups.Subsystem{cgroups.NewMemory(t.TempDir())},
		func(string, *unix.Statfs_t) error { return os.ErrPermission })
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("permission failure lost: %v", err)
	}
}

func writeCgroupFixture(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cgroup")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPidPathFromFile(t *testing.T) {
	p := writeCgroupFixture(t, "6:memory:/target\n3:cpu,cpuacct:/target\n1:name=systemd:/target\n")
	resolve := pidPathFromFile(p)
	for _, name := range []cgroups.Name{cgroups.Memory, cgroups.Cpu, cgroups.Cpuacct, cgroups.Name("systemd")} {
		got, err := resolve(name)
		if err != nil || got != "/target" {
			t.Errorf("%s: path %q, error %v", name, got, err)
		}
	}
	if _, err := resolve(cgroups.Name("hugetlb")); err != cgroups.ErrControllerNotActive {
		t.Fatalf("expected skippable inactive controller, got %v", err)
	}
}

func TestPidPathReadErrors(t *testing.T) {
	for _, p := range []string{
		filepath.Join(t.TempDir(), "missing"),
		writeCgroupFixture(t, "invalid cgroup line\n"),
	} {
		_, err := pidPathFromFile(p)(cgroups.Memory)
		if err == nil || err == cgroups.ErrControllerNotActive {
			t.Errorf("proc read/parse error hidden for %s: %v", p, err)
		}
	}
}

func TestLoadSkipsControllerAbsentFromTarget(t *testing.T) {
	root := t.TempDir()
	p := writeCgroupFixture(t, "6:memory:/target\n")
	if err := os.MkdirAll(filepath.Join(root, "memory", "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A mounted controller can still be absent from the target PID. The real
	// loader must recognize the inactive sentinel and keep loading memory.
	hierarchy := func() ([]cgroups.Subsystem, error) {
		return []cgroups.Subsystem{cgroups.NewNamed(root, "hugetlb"), cgroups.NewMemory(root)}, nil
	}
	group, err := cgroups.Load(hierarchy, pidPathFromFile(p))
	if err != nil {
		t.Fatal(err)
	}
	if got := group.Subsystems(); len(got) != 1 || got[0].Name() != cgroups.Memory {
		t.Fatalf("unexpected active subsystems: %v", got)
	}
}

func TestLoadFailsWhenNoControllerRemains(t *testing.T) {
	p := writeCgroupFixture(t, "6:memory:/target\n")
	hierarchy := func() ([]cgroups.Subsystem, error) {
		return []cgroups.Subsystem{cgroups.NewNamed(t.TempDir(), "hugetlb")}, nil
	}
	if _, err := cgroups.Load(hierarchy, pidPathFromFile(p)); err != cgroups.ErrCgroupDeleted {
		t.Fatalf("expected failure without usable controllers, got %v", err)
	}
}

func TestLoadV1ForExperimentRequiredControllers(t *testing.T) {
	// pids is always present so the failure is specifically the missing
	// required controller, not an empty hierarchy. Each missing source must be
	// checked via the final load result: proc data or mounts alone are not
	// sufficient.
	for _, tc := range []struct {
		target  string
		missing cgroups.Name
	}{
		{"mem", cgroups.Memory},
		{"cpu", cgroups.Cpu},
		{"cpu", cgroups.Cpuacct},
	} {
		for _, source := range []string{"hierarchy", "pid", "directory"} {
			t.Run(tc.target+"/"+string(tc.missing)+"/"+source, func(t *testing.T) {
				root := t.TempDir()
				var subsystems []cgroups.Subsystem
				var proc strings.Builder
				for i, name := range []cgroups.Name{cgroups.Memory, cgroups.Cpu, cgroups.Cpuacct, cgroups.Pids} {
					if name != tc.missing || source != "hierarchy" {
						subsystems = append(subsystems, cgroups.NewNamed(root, name))
					}
					if name != tc.missing || source != "pid" {
						fmt.Fprintf(&proc, "%d:%s:/target\n", i+1, name)
					}
					if name != tc.missing || source != "directory" {
						if err := os.MkdirAll(filepath.Join(root, string(name), "target"), 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
				hierarchy := func() ([]cgroups.Subsystem, error) { return subsystems, nil }
				p := pidPathFromFile(writeCgroupFixture(t, proc.String()))
				group, err := LoadV1ForExperiment(hierarchy, p, tc.target)
				if group != nil || err == nil || !strings.Contains(err.Error(), "requires active controller(s): "+string(tc.missing)) {
					t.Fatalf("expected missing %s error, got %v, %v", tc.missing, group, err)
				}
			})
		}
	}
}

func TestLoadV1ForExperimentOptionalControllers(t *testing.T) {
	for _, tc := range []struct {
		target string
		names  []string
	}{
		{"mem", []string{"memory"}},
		{"cpu", []string{"cpu", "cpuacct"}},
		{"disk", []string{"pids"}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			root := t.TempDir()
			subsystems := []cgroups.Subsystem{cgroups.NewNamed(root, "hugetlb")}
			var proc strings.Builder
			for i, name := range tc.names {
				subsystems = append(subsystems, cgroups.NewNamed(root, cgroups.Name(name)))
				fmt.Fprintf(&proc, "%d:%s:/target\n", i+1, name)
				if err := os.MkdirAll(filepath.Join(root, name, "target"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			hierarchy := func() ([]cgroups.Subsystem, error) { return subsystems, nil }
			// hugetlb is absent from the target PID and must stay skippable.
			group, err := LoadV1ForExperiment(hierarchy, pidPathFromFile(writeCgroupFixture(t, proc.String())), tc.target)
			if err != nil || len(group.Subsystems()) != len(tc.names) {
				t.Fatalf("optional controller prevented loading: %v, %v", group, err)
			}
		})
	}
}

type statsFixture struct {
	cgroups.Cgroup
	subsystems []cgroups.Subsystem
	stats      *v1.Metrics
	err        error
}

func (g statsFixture) Subsystems() []cgroups.Subsystem { return g.subsystems }

func (g statsFixture) Stat(handlers ...cgroups.ErrorHandler) (*v1.Metrics, error) {
	// Mirror containerd's error handling: IgnoreNotExist would swallow a
	// vanished file, so this fixture detects a regression that reintroduces
	// silent zero usage.
	err := g.err
	for _, handler := range handlers {
		if err != nil {
			err = handler(err)
		}
	}
	return g.stats, err
}

func TestStatV1ForExperimentMissingFields(t *testing.T) {
	validMemory := &v1.Metrics{Memory: &v1.MemoryStat{Usage: &v1.MemoryEntry{Limit: 1024}}}
	validCPU := &v1.Metrics{CPU: &v1.CPUStat{Usage: &v1.CPUUsage{}}}
	for _, tc := range []struct {
		name   string
		target string
		stats  *v1.Metrics
		ok     bool
	}{
		{"nil_memory_stats", "mem", nil, false},
		{"nil_memory", "mem", &v1.Metrics{}, false},
		{"nil_memory_usage", "mem", &v1.Metrics{Memory: &v1.MemoryStat{}}, false},
		{"valid_memory_zero_usage", "mem", validMemory, true},
		{"nil_cpu_stats", "cpu", nil, false},
		{"nil_cpu", "cpu", &v1.Metrics{}, false},
		{"nil_cpu_usage", "cpu", &v1.Metrics{CPU: &v1.CPUStat{}}, false},
		{"valid_cpu_zero_usage", "cpu", validCPU, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := statsFixture{
				subsystems: []cgroups.Subsystem{cgroups.NewMemory(""), cgroups.NewCpu(""), cgroups.NewCpuacct("")},
				stats:      tc.stats,
			}
			got, err := StatV1ForExperiment(g, tc.target)
			if tc.ok {
				if err != nil || got != tc.stats {
					t.Fatalf("valid statistics rejected: %v, %v", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatalf("missing statistics accepted: %v, %v", got, err)
			}
		})
	}
}

func TestStatV1ForExperimentReadErrors(t *testing.T) {
	for _, cause := range []error{os.ErrNotExist, os.ErrPermission} {
		for _, target := range []string{"mem", "cpu"} {
			g := statsFixture{
				subsystems: []cgroups.Subsystem{cgroups.NewMemory(""), cgroups.NewCpu(""), cgroups.NewCpuacct("")},
				stats: &v1.Metrics{
					Memory: &v1.MemoryStat{Usage: &v1.MemoryEntry{}},
					CPU:    &v1.CPUStat{Usage: &v1.CPUUsage{}},
				},
				err: cause,
			}
			if got, err := StatV1ForExperiment(g, target); got != nil || !errors.Is(err, cause) {
				t.Fatalf("%s: read failure hidden: %v, %v", target, got, err)
			}
		}
	}
}

func TestStatV1ForExperimentMissingFiles(t *testing.T) {
	for _, target := range []string{"cpu", "mem"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"cpu", "cpuacct", "memory"} {
				if err := os.MkdirAll(filepath.Join(root, name, "target"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			// cpu uses named controllers to avoid unrelated file errors;
			// memory/cpuacct use the real readers. A loadable directory with
			// missing statistics files must fail instead of reporting zero.
			hierarchy := func() ([]cgroups.Subsystem, error) {
				if target == "mem" {
					return []cgroups.Subsystem{cgroups.NewMemory(root)}, nil
				}
				return []cgroups.Subsystem{cgroups.NewNamed(root, "cpu"), cgroups.NewCpuacct(root)}, nil
			}
			group, err := LoadV1ForExperiment(hierarchy, cgroups.StaticPath("/target"), target)
			if err != nil {
				t.Fatal(err)
			}
			if stats, err := StatV1ForExperiment(group, target); stats != nil || !os.IsNotExist(errors.Unwrap(err)) {
				t.Fatalf("missing statistics files accepted: %v, %v", stats, err)
			}
		})
	}
}
