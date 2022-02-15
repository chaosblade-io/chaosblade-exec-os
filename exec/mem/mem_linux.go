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
	"fmt"
	"github.com/containerd/cgroups"
	"github.com/shirou/gopsutil/mem"
)

func getAvailableAndTotal(burnMemMode string, includeBufferCache bool) (int64, int64, error) {
	cgroup, err := cgroups.Load(cgroups.V1, cgroups.StaticPath("/"))
	if err != nil {
		return 0, 0, fmt.Errorf("load cgroup error, %v", err)
	}
	stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return 0, 0, fmt.Errorf("load cgroup stat error, %v", err)
	}

	total := int64(0)
	available := int64(0)

	if stats == nil || stats.Memory.Usage.Limit >= PageCounterMax {
		//no limit
		virtualMemory, err := mem.VirtualMemory()
		if err != nil {
			return 0, 0, err
		}
		total = int64(virtualMemory.Total)
		available = int64(virtualMemory.Free)
		if burnMemMode == "ram" && !includeBufferCache {
			available = available + int64(virtualMemory.Buffers+virtualMemory.Cached)
		}
	} else {
		total = int64(stats.Memory.Usage.Limit)
		available = total - int64(stats.Memory.Usage.Usage)
		if burnMemMode == "ram" && !includeBufferCache {
			available = available + int64(stats.Memory.Cache)
		}
	}
	return total, available, nil
}
