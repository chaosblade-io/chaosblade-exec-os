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
    "github.com/chaosblade-io/chaosblade-exec-os/exec"
    "github.com/chaosblade-io/chaosblade-spec-go/channel"
    "github.com/chaosblade-io/chaosblade-spec-go/log"
    "github.com/containerd/cgroups"
    "github.com/shirou/gopsutil/mem"
    "strconv"
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

        log.Debugf(ctx, "get mem useage by cgroup, root path: %s", cgroupRoot)

        cgroup, err := cgroups.Load(exec.Hierarchy(cgroupRoot.(string)), exec.PidPath(p))
        if err != nil {
            return 0, 0, fmt.Errorf("load cgroup error, %v", err)
        }
        stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
        if err != nil {
            return 0, 0, fmt.Errorf("load cgroup stat error, %v", err)
        }
        if stats != nil && stats.Memory.Usage.Limit < PageCounterMax {
            total = int64(stats.Memory.Usage.Limit)
            available = total - int64(stats.Memory.Usage.Usage)
            if burnMemMode == "ram" && !includeBufferCache {
                available = available + int64(stats.Memory.Cache)
            }
            return total, available, nil
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
