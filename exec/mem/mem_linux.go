/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
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

package mem

import (
	"context"
	"fmt"
	"strconv"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/containerd/cgroups"
	"github.com/shirou/gopsutil/mem"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	cgroupsv2 "github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
)

func getAvailableAndTotal(ctx context.Context, burnMemMode string, includeBufferCache bool) (int64, int64, error) {
	pid := ctx.Value(channel.NSTargetFlagName)
	total := int64(0)
	available := int64(0)

	if pid != nil {
		p, err := strconv.Atoi(pid.(string))
		if err != nil {
			return 0, 0, fmt.Errorf("load cgroup error, %v", err)
		}

		cgroupRoot := ctx.Value("cgroup-root")
		if cgroupRoot == "" {
			cgroupRoot = "/sys/fs/cgroup/"
		}

		log.Debugf(ctx, "get mem usage by cgroup, root path: %s", cgroupRoot)

		// 检测 cgroup 版本
		version := cgroupsv2.DetectCGroupVersion(ctx, cgroupRoot.(string))

		switch version {
		case cgroupsv2.CGroupV2:
			log.Infof(ctx, "detected cgroup v2, using v2 memory implementation")
			return getAvailableAndTotalV2(ctx, burnMemMode, includeBufferCache)
		case cgroupsv2.CGroupV1:
			log.Infof(ctx, "detected cgroup v1, using v1 memory implementation")
			return getAvailableAndTotalV1(ctx, burnMemMode, includeBufferCache, p, cgroupRoot.(string))
		default:
			log.Warnf(ctx, "unknown cgroup version, falling back to v1 implementation")
			return getAvailableAndTotalV1(ctx, burnMemMode, includeBufferCache, p, cgroupRoot.(string))
		}
	}

	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, err
	}
	total = int64(virtualMemory.Total)
	available = int64(virtualMemory.Free)
	if burnMemMode == "ram" && !includeBufferCache {
		available = available + int64(virtualMemory.Buffers+virtualMemory.Cached)
	}
	return total, available, nil
}

// getAvailableAndTotalV1 获取 cgroup v1 环境下的可用和总内存
func getAvailableAndTotalV1(ctx context.Context, burnMemMode string, includeBufferCache bool, p int, cgroupRoot string) (int64, int64, error) {
	cgroup, err := cgroups.Load(exec.Hierarchy(cgroupRoot), exec.PidPath(p))
	if err != nil {
		return 0, 0, fmt.Errorf("load cgroup error, %v", err)
	}
	stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return 0, 0, fmt.Errorf("load cgroup stat error, %v", err)
	}
	if stats != nil && stats.Memory.Usage.Limit < PageCounterMax {
		total := int64(stats.Memory.Usage.Limit)
		available := total - int64(stats.Memory.Usage.Usage)
		if burnMemMode == "ram" && !includeBufferCache {
			available = available + int64(stats.Memory.Cache)
		}
		return total, available, nil
	}

	// 回退到系统内存
	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, err
	}
	total := int64(virtualMemory.Total)
	available := int64(virtualMemory.Free)
	if burnMemMode == "ram" && !includeBufferCache {
		available = available + int64(virtualMemory.Buffers+virtualMemory.Cached)
	}
	return total, available, nil
}
