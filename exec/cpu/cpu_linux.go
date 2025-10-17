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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	containerdCgroups "github.com/containerd/cgroups"
	"github.com/shirou/gopsutil/cpu"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs/cgroups"
)

// getCGroupV2CPUUsage 获取 cgroup v2 环境下的 CPU 使用率
func getCGroupV2CPUUsage(ctx context.Context, cgroupPath string, cpuCount int) (float64, error) {
	// 读取 cpu.stat 文件获取 CPU 使用统计
	cpuStatFile := filepath.Join(cgroupPath, "cpu.stat")
	content, err := os.ReadFile(cpuStatFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read cpu.stat file: %v", err)
	}

	// 解析 cpu.stat 文件内容
	lines := strings.Split(string(content), "\n")
	var usageUsec int64
	var userUsec int64
	var systemUsec int64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}

		switch parts[0] {
		case "usage_usec":
			usageUsec = value
		case "user_usec":
			userUsec = value
		case "system_usec":
			systemUsec = value
		}
	}

	// 计算第一次的 CPU 使用时间（微秒）
	firstTotal := usageUsec
	if firstTotal == 0 {
		// 如果没有 usage_usec，使用 user_usec + system_usec
		firstTotal = userUsec + systemUsec
	}

	// 等待 1 秒
	time.Sleep(time.Second)

	// 再次读取 cpu.stat 文件
	content, err = os.ReadFile(cpuStatFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read cpu.stat file again: %v", err)
	}

	// 重新解析
	lines = strings.Split(string(content), "\n")
	usageUsec = 0
	userUsec = 0
	systemUsec = 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}

		switch parts[0] {
		case "usage_usec":
			usageUsec = value
		case "user_usec":
			userUsec = value
		case "system_usec":
			systemUsec = value
		}
	}

	// 计算第二次的 CPU 使用时间（微秒）
	secondTotal := usageUsec
	if secondTotal == 0 {
		secondTotal = userUsec + systemUsec
	}

	// 计算 CPU 使用率
	// 时间差（微秒）转换为秒，然后除以 CPU 核心数
	timeDiff := float64(secondTotal-firstTotal) / 1000000.0 // 转换为秒
	cpuUsage := (timeDiff * 100.0) / float64(cpuCount)

	log.Debugf(ctx, "cgroup v2 cpu usage: first=%d, second=%d, diff=%f, cpuCount=%d, usage=%f%%",
		firstTotal, secondTotal, timeDiff, cpuCount, cpuUsage)

	return cpuUsage, nil
}

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
			cgroupRoot = "/sys/fs/cgroup"
		}

		log.Debugf(ctx, "get cpu usage by cgroup, root path: %s", cgroupRoot)

		// 首先尝试 cgroup v2
		cgroupPath, err := cgroups.FindCGroupV2Path(ctx, strconv.Itoa(p), cgroupRoot.(string))
		if err == nil && cgroupPath != "" {
			log.Debugf(ctx, "using cgroup v2 path: %s", cgroupPath)
			cpuUsage, err := getCGroupV2CPUUsage(ctx, cgroupPath, cpuCount)
			if err != nil {
				log.Errorf(ctx, "failed to get cgroup v2 cpu usage: %v, falling back to cgroup v1", err)
			} else {
				return cpuUsage
			}
		} else {
			log.Debugf(ctx, "cgroup v2 not available, trying cgroup v1: %v", err)
		}

		// 回退到 cgroup v1
		cgroup, err := containerdCgroups.Load(exec.Hierarchy(cgroupRoot.(string)), exec.PidPath(p))
		if err != nil {
			log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
		}

		stats, err := cgroup.Stat(containerdCgroups.IgnoreNotExist)
		if err != nil {
			log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
		} else {
			pre := float64(stats.CPU.Usage.Total) / float64(time.Second)
			time.Sleep(time.Second)
			nextStats, err := cgroup.Stat(containerdCgroups.IgnoreNotExist)
			if err != nil {
				log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
			} else {
				next := float64(nextStats.CPU.Usage.Total) / float64(time.Second)
				return ((next - pre) * 100) / float64(cpuCount)
			}
		}
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
