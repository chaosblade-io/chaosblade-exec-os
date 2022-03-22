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

package cpu

import (
	"context"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/containerd/cgroups"
	"github.com/shirou/gopsutil/cpu"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

func getUsed(ctx context.Context, cpuCount int) float64 {

	pid := ctx.Value(channel.NSTargetFlagName)

	if pid != nil {
		p, err := strconv.Atoi(pid.(string))
		if err != nil {
			logrus.Fatalf("get cpu usage fail, %s", err.Error())
		}

		cgroupRoot := ctx.Value("cgroup-root")
		logrus.Debugf("get cpu useage by cgroup, root path: %s", cgroupRoot)

		cgroup, err := cgroups.Load(exec.Hierarchy(cgroupRoot.(string)), exec.PidPath(p))
		if err != nil {
			logrus.Fatalf("get cpu usage fail, %s", err.Error())
		}

		stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
		if err != nil {
			logrus.Fatalf("get cpu usage fail, %s", err.Error())
		} else {
			pre := float64(stats.CPU.Usage.Total) / float64(time.Second)
			time.Sleep(time.Second)
			nextStats, err := cgroup.Stat(cgroups.IgnoreNotExist)
			if err != nil {
				logrus.Fatalf("get cpu usage fail, %s", err.Error())
			} else {
				next := float64(nextStats.CPU.Usage.Total) / float64(time.Second)
				return ((next - pre) * 100) / float64(cpuCount)
			}
		}
	}

	totalCpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		logrus.Fatalf("get cpu usage fail, %s", err.Error())
	}
	return totalCpuPercent[0]
}
