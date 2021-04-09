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

package burnmem

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/containerd/cgroups"
	v1 "github.com/containerd/cgroups/stats/v1"
	"github.com/shirou/gopsutil/mem"

	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(BurnMem))
}

type BurnMem struct {
	BurnMemStart       bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start burn memory"`
	BurnMemStop        bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop burn memory"`
	BurnMemNohup       bool   `name:"nohup" json:"nohup" yaml:"nohup" default:"false" help:"nohup to run burn memory"`
	IncludeBufferCache bool   `name:"include-buffer-cache" json:"include-buffer-cache" yaml:"include-buffer-cache" default:"false" help:"ram model mem-percent is exclude buffer/cache"`
	MemPercent         int    `name:"mem-percent" json:"mem-percent" yaml:"mem-percent" default:"0" help:"percent of burn memory"`
	MemReserve         int    `name:"reserve" json:"reserve" yaml:"reserve" default:"0" help:"reserve to burn memory, unit is M"`
	MemRate            int    `name:"rate" json:"rate" yaml:"rate" default:"100" help:"burn memory rate, unit is M/S, only support for ram mode"`
	BurnMemMode        string `name:"mode" json:"mode" yaml:"mode" default:"cache" help:"burn memory mode, cache or ram"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
	StopBurnMem func() (success bool, errs string)                                                                          `kong:"-"`
	RunBurnMem  func(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string, includeBufferCache bool) `kong:"-"`
}

func (that *BurnMem) Assign() model.Worker {
	worker := &BurnMem{Channel: channel.NewLocalChannel()}
	worker.StopBurnMem = func() (success bool, errs string) {
		return worker.stopBurnMem()
	}
	worker.RunBurnMem = func(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string, includeBufferCache bool) {
		worker.runBurnMem(ctx, memPercent, memReserve, memRate, burnMemMode, includeBufferCache)
	}
	return worker
}

func (that *BurnMem) Name() string {
	return exec.BurnMemBin
}

func (that *BurnMem) Exec() *spec.Response {
	if that.BurnMemStart {
		that.startBurnMem()
	} else if that.BurnMemStop {
		if success, errs := that.stopBurnMem(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if that.BurnMemNohup {
		if that.BurnMemMode == "cache" {
			that.burnMemWithCache()
		} else if that.BurnMemMode == "ram" {
			that.burnMemWithRam()
		}
	} else {
		bin.PrintAndExitWithErrPrefix("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

const PageCounterMax uint64 = 9223372036854770000

// 128K
type Block [32 * 1024]int32

var dirName = "burnmem_tmpfs"

var fileName = "file"

var fileCount = 1

func (that *BurnMem) burnMemWithRam() {
	tick := time.Tick(time.Second)
	var cache = make(map[int][]Block, 1)
	var count = 1
	cache[count] = make([]Block, 0)
	if that.MemRate <= 0 {
		that.MemRate = 100
	}
	for range tick {
		_, expectMem, err := that.calculateMemSize(that.MemPercent, that.MemReserve)
		if err != nil {
			that.StopBurnMem()
			bin.PrintErrAndExit(err.Error())
		}
		fillMem := expectMem
		if expectMem > 0 {
			if expectMem > int64(that.MemRate) {
				fillMem = int64(that.MemRate)
			} else {
				fillMem = expectMem / 10
				if fillMem == 0 {
					continue
				}
			}
			fillSize := int(8 * fillMem)
			buf := cache[count]
			if cap(buf)-len(buf) < fillSize &&
				int(math.Floor(float64(cap(buf))*1.25)) >= int(8*expectMem) {
				count += 1
				cache[count] = make([]Block, 0)
				buf = cache[count]
			}
			logrus.Debugf("count: %d, len(buf): %d, cap(buf): %d, expect mem: %d, fill size: %d",
				count, len(buf), cap(buf), expectMem, fillSize)
			cache[count] = append(buf, make([]Block, fillSize)...)
		}
	}
}

func (that *BurnMem) burnMemWithCache() {
	filePath := path.Join(path.Join(util.GetProgramPath(), dirName), fileName)
	tick := time.Tick(time.Second)
	for range tick {
		_, expectMem, err := that.calculateMemSize(that.MemPercent, that.MemReserve)
		if err != nil {
			that.StopBurnMem()
			bin.PrintErrAndExit(err.Error())
		}
		fillMem := expectMem
		if expectMem > 0 {
			if expectMem > int64(that.MemRate) {
				fillMem = int64(that.MemRate)
			}
			nFilePath := fmt.Sprintf("%s%d", filePath, fileCount)
			response := that.Channel.Run(context.Background(), "dd", fmt.Sprintf("if=/dev/zero of=%s bs=1M count=%d", nFilePath, fillMem))
			if !response.Success {
				that.StopBurnMem()
				bin.PrintErrAndExit(response.Error())
			}
			fileCount++
		}
	}
}

func (that *BurnMem) startBurnMem() {
	ctx := context.Background()
	if that.BurnMemMode == "cache" {
		flPath := path.Join(util.GetProgramPath(), dirName)
		if _, err := os.Stat(flPath); err != nil {
			err = os.Mkdir(flPath, os.ModePerm)
			if err != nil {
				bin.PrintErrAndExit(err.Error())
			}
		}
		response := that.Channel.Run(ctx, "mount", fmt.Sprintf("-t tmpfs tmpfs %s -o size=", flPath)+"100%")
		if !response.Success {
			bin.PrintErrAndExit(response.Error())
		}
	}
	that.RunBurnMem(ctx, that.MemPercent, that.MemReserve, that.MemRate, that.BurnMemMode, that.IncludeBufferCache)
}

func (that *BurnMem) runBurnMem(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string, includeBufferCache bool) {
	args := fmt.Sprintf(`%s --nohup --mem-percent %d --reserve %d --rate %d --mode %s --include-buffer-cache=%t`,
		path.Join(util.GetProgramPath(), that.Name()), memPercent, memReserve, memRate, burnMemMode, includeBufferCache)
	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := that.Channel.Run(ctx, "nohup", args)
	if !response.Success {
		that.StopBurnMem()
		bin.PrintErrAndExit(response.Err)
	}
	// check pid
	newCtx := context.WithValue(context.Background(), channel.ProcessKey, "--nohup")
	pids, err := that.Channel.GetPidsByProcessName(that.Name(), newCtx)
	if err != nil {
		that.StopBurnMem()
		bin.PrintErrAndExit(fmt.Sprintf("run burn memory by %s mode failed, cannot get the burning program pid, %v",
			burnMemMode, err))
	}
	if len(pids) == 0 {
		that.StopBurnMem()
		bin.PrintErrAndExit(fmt.Sprintf("run burn memory by %s mode failed, cannot find the burning program pid",
			burnMemMode))
	}
}

func (that *BurnMem) stopBurnMem() (success bool, errs string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey, "nohup")
	ctx = context.WithValue(ctx, channel.ExcludeProcessKey, "stop")
	pids, _ := that.Channel.GetPidsByProcessName(that.Name(), ctx)
	var response *spec.Response
	if pids != nil && len(pids) != 0 {
		response = that.Channel.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
		if !response.Success {
			return false, response.Err
		}
	}
	if that.BurnMemMode == "cache" {
		dirPath := path.Join(util.GetProgramPath(), dirName)
		if _, err := os.Stat(dirPath); err == nil {
			response = that.Channel.Run(ctx, "umount", dirPath)
			if !response.Success {
				if !strings.Contains(response.Err, "not mounted") {
					bin.PrintErrAndExit(response.Error())
				}
			}
			err = os.RemoveAll(dirPath)
			if err != nil {
				bin.PrintErrAndExit(err.Error())
			}
		}
	}
	return true, errs
}

func (that *BurnMem) calculateMemSize(percent, reserve int) (int64, int64, error) {
	total := int64(0)
	available := int64(0)
	memoryStat, err := getMemoryStatsByCGroup()
	if err != nil {
		logrus.Infof("get memory stats by cgroup failed, used proc memory, %v", err)
	}
	if memoryStat == nil || memoryStat.Usage.Limit >= PageCounterMax {
		//no limit
		virtualMemory, err := mem.VirtualMemory()
		if err != nil {
			return 0, 0, err
		}
		total = int64(virtualMemory.Total)
		available = int64(virtualMemory.Free)
		if that.BurnMemMode == "ram" && !that.IncludeBufferCache {
			available = available + int64(virtualMemory.Buffers + virtualMemory.Cached)
		}
	} else {
		total = int64(memoryStat.Usage.Limit)
		available = total - int64(memoryStat.Usage.Usage)
		if that.BurnMemMode == "ram" && !that.IncludeBufferCache {
			available = available + int64(memoryStat.Cache)
		}
	}
	reserved := int64(0)
	if percent != 0 {
		reserved = (total * int64(100-percent) / 100) / 1024 / 1024
	} else {
		reserved = int64(reserve)
	}
	expectSize := available/1024/1024 - reserved

	logrus.Debugf("available: %d, percent: %d, reserved: %d, expectSize: %d",
		available/1024/1024, percent, reserved, expectSize)

	return total / 1024 / 1024, expectSize, nil
}

func getMemoryStatsByCGroup() (*v1.MemoryStat, error) {
	cgroup, err := cgroups.Load(cgroups.V1, cgroups.StaticPath("/"))
	if err != nil {
		return nil, fmt.Errorf("load cgroup error, %v", err)
	}
	stats, err := cgroup.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return nil, fmt.Errorf("load cgroup stat error, %v", err)
	}
	return stats.Memory, nil
}
