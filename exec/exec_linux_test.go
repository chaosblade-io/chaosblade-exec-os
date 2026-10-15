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
//
// Approach: mount types are injected so no host mount changes are needed.
// Proc files and group directories use temp dirs, while loading goes through
// the project-pinned real containerd cgroups library.
package exec

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/cgroups"
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
