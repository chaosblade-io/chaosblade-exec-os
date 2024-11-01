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
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/shirou/gopsutil/cpu"
	"os"
	"strconv"
	"time"
)

func getUsed(ctx context.Context, percpu bool, cpuIndex int) float64 {

	pid := ctx.Value(channel.NSTargetFlagName)
	cpuCount := ctx.Value("cpuCount").(int)

	if pid != nil {
		p, err := strconv.Atoi(pid.(string))
		if err != nil {
			log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
		}

		cgroupRoot := ctx.Value("cgroup-root")
		if cgroupRoot == "" {
			cgroupRoot = "/sys/fs/cgroup/"
		}

		v2 := isCGroupV2(cgroupRoot.(string))
		if v2 {
			g, err := cgroupsv2.PidGroupPath(p)
			if err != nil {
				sprintf := fmt.Sprintf("loading cgroup2 for %d, err ", pid, err.Error())
				log.Fatalf(ctx, "get cpu usage fail, %s", sprintf)
			}
			cg, err := cgroupsv2.LoadManager("/sys/fs/cgroup/", g)
			if err != nil {
				if err != cgroupsv2.ErrCgroupDeleted {
					sprintf := fmt.Sprintf("cgroups V2 load failed, %s", err.Error())
					log.Fatalf(ctx, "get cpu usage fail, %s", sprintf)
				}
				if cg, err = cgroupsv2.NewManager("/sys/fs/cgroup", cgroupRoot.(string), nil); err != nil {
					sprintf := fmt.Sprintf("cgroups V2 new manager failed, %s", err.Error())
					log.Fatalf(ctx, "get cpu usage fail, %s", sprintf)
				}
			}
			stats, err := cg.Stat()
			if err != nil {
				log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
			} else {
				pre := float64(stats.CPU.UsageUsec) / float64(time.Second)
				time.Sleep(time.Second)
				nextStats, err := cg.Stat()
				if err != nil {
					log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
				} else {
					next := float64(nextStats.CPU.UsageUsec) / float64(time.Second)
					return ((next - pre) * 100) / float64(cpuCount)
				}
			}
		} else {

			cgroup, err := cgroups.Load(exec.Hierarchy(cgroupRoot.(string)), exec.PidPath(p))
			if err != nil {
				log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
			}

			stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
			if err != nil {
				log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
			} else {
				pre := float64(stats.CPU.Usage.Total) / float64(time.Second)
				time.Sleep(time.Second)
				nextStats, err := cgroup.Stat(cgroups.IgnoreNotExist)
				if err != nil {
					log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
				} else {
					next := float64(nextStats.CPU.Usage.Total) / float64(time.Second)
					return ((next - pre) * 100) / float64(cpuCount)
				}
			}
		}

		log.Debugf(ctx, "get cpu useage by cgroup, root path: %s", cgroupRoot)
	}

	totalCpuPercent, err := cpu.Percent(time.Second, percpu)
	if err != nil {
		log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
	}
	if percpu {
		if cpuIndex > len(totalCpuPercent) {
			log.Fatalf(ctx, "illegal cpu index %d", cpuIndex)
		}
		return totalCpuPercent[cpuIndex]
	}
	return totalCpuPercent[0]
}

func isCGroupV2(cgroupRoot string) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/cgroup.controllers", cgroupRoot)); err == nil {
		return true
	}
	return false
}
