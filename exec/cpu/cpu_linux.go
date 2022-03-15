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
	"errors"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/containerd/cgroups"
	"github.com/shirou/gopsutil/cpu"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

// todo
func getUsed(ctx context.Context, cpuCount int) float64 {

	pid := ctx.Value(channel.NSTargetFlagName)

	if pid != nil {
		p, err := strconv.Atoi(pid.(string))
		if err != nil {
			logrus.Fatalf("get cpu usage fail, %s", err.Error())
		}

		logrus.Debugf("get cpu useage by cgroup ")
		cgroup, err := cgroups.Load(cgroups.V1, pidPath(p))
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

func pidPath(pid int) cgroups.Path {
	p := fmt.Sprintf("/proc/%d/cgroup", pid)
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
				return "", errors.New("controller is not supported")
			}

		}
		return root, nil
	}
}

func getProcessComm(pid int) (string, error) {
	f, err := os.Open(fmt.Sprintf("%s/%d/comm", "/proc", pid))
	if err != nil {
		return "", err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
