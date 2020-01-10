/*
 * Copyright 1999-2019 Alibaba Group Holding Ltd.
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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	v1 "github.com/containerd/cgroups/stats/v1"
	"github.com/shirou/gopsutil/mem"

	cl "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

const PAGE_COUNTER_MAX uint64 = 9223372036854771712

//32K
type Block [8 * 1024]int32

var (
	burnMemStart, burnMemStop, burnMemNohup bool
	memPercent, memReserve, memRate         int
	burnMemMode                             string
)

func main() {
	flag.BoolVar(&burnMemStart, "start", false, "start burn memory")
	flag.BoolVar(&burnMemStop, "stop", false, "stop burn memory")
	flag.BoolVar(&burnMemNohup, "nohup", false, "nohup to run burn memory")
	flag.IntVar(&memPercent, "mem-percent", 0, "percent of burn memory")
	flag.IntVar(&memReserve, "reserve", 0, "reserve to burn memory, unit is M")
	flag.IntVar(&memRate, "rate", 0, "burn memory rate, unit is M/S, only support for ram mode")
	flag.StringVar(&burnMemMode, "mode", "cache", "burn memory mode, cache or ram")
	bin.ParseFlagAndInitLog()

	if burnMemStart {
		startBurnMem()
	} else if burnMemStop {
		if success, errs := stopBurnMem(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if burnMemNohup {
		if burnMemMode == "cache" {
			burnMemWithCache()
		} else if burnMemMode == "ram" {
			burnMemWithRam()
		}

	} else {
		bin.PrintAndExitWithErrPrefix("less --start or --stop flag")
	}

}

var dirName = "burnmem_tmpfs"

var fileName = "file"

var fileCount = 0

func getMem(filePath string) int64 {
	sum := int64(0)
	if 0 == fileCount {
		return sum
	}
	fileInfo, err := os.Stat(filePath + strconv.Itoa(fileCount-1))
	if err != nil {
		bin.PrintErrAndExit(err.Error())
	}
	sum += fileInfo.Size()/1024/1024 + int64(fileCount-1)*128
	return sum
}

func burnMemWithRam() {
	total, _, err := calculateMemSize(memPercent, memReserve)
	if err != nil {
		bin.PrintErrAndExit(err.Error())
	}
	buffChan := make(chan Block, total*1024/32)
	defer close(buffChan)
	for {
		t := time.NewTicker(1 * time.Second)
		select {
		case <-t.C:
			_, expectMem, err := calculateMemSize(memPercent, memReserve)
			if err != nil {
				bin.PrintErrAndExit(err.Error())
			}
			if expectMem > 0 {
				if expectMem > int64(memRate) && memRate > 0 {
					fillBuffChan(int64(memRate), buffChan)
				} else {
					fillBuffChan(expectMem, buffChan)
				}
			}
		}
	}
}

func fillBuffChan(memSize int64, buffChan chan Block) {
	for i := int64(0); i < memSize*1024/32; i++ {
		buffChan <- Block{0}
	}
}

func burnMemWithCache() {
	filePath := path.Join(path.Join(util.GetProgramPath(), dirName), fileName)
	go func() {
		t := time.NewTicker(3 * time.Second)
		for {
			select {
			case <-t.C:
				memSum := getMem(filePath)
				_, expectMem, err := calculateMemSize(memPercent, memReserve)
				if err != nil {
					bin.PrintErrAndExit(err.Error())
				}

				needMem := expectMem + memSum
				if needMem <= 0 {
					for i := 0; i < fileCount; i++ {
						os.Remove(filePath + strconv.Itoa(i))
					}
					fileCount = 0
				} else {
					if memSum%128 != 0 {
						os.Remove(filePath + strconv.Itoa(fileCount-1))
						memSum -= memSum % 128
						fileCount--
					}
					if needMem/128 > memSum/128 {
						for i := memSum / 128; i < needMem/128; i++ {
							nFilePath := filePath + strconv.FormatInt(i, 10)
							response := channel.Run(context.Background(), "dd", fmt.Sprintf("if=/dev/zero of=%s bs=1M count=%d", nFilePath, 128))
							if !response.Success {
								bin.PrintErrAndExit(response.Error())
							}
						}
					} else {
						for i := needMem / 128; i < memSum/128; i++ {
							nFilePath := filePath + strconv.FormatInt(i, 10)
							os.RemoveAll(nFilePath)
						}
					}
					fileCount = int(needMem / 128)
					if needMem%128 != 0 {
						nFilePath := filePath + strconv.Itoa(fileCount)
						response := channel.Run(context.Background(), "dd", fmt.Sprintf("if=/dev/zero of=%s bs=1M count=%d", nFilePath, needMem%128))
						if !response.Success {
							bin.PrintErrAndExit(response.Error())
						}
						fileCount++
					}
				}
			}
		}
	}()
	select {}
}

var burnMemBin = "chaos_burnmem"

var channel = cl.NewLocalChannel()

var stopBurnMemFunc = stopBurnMem

var runBurnMemFunc = runBurnMem

func startBurnMem() {
	ctx := context.Background()
	if burnMemMode == "cache" {
		flPath := path.Join(util.GetProgramPath(), dirName)
		if _, err := os.Stat(flPath); err != nil {
			err = os.Mkdir(flPath, os.ModePerm)
			if err != nil {
				bin.PrintErrAndExit(err.Error())
			}
		}
		response := channel.Run(ctx, "mount", fmt.Sprintf("-t tmpfs tmpfs %s -o size=", flPath)+"100%")
		if !response.Success {
			bin.PrintErrAndExit(response.Error())
		}
	}
	runBurnMemFunc(ctx, memPercent, memReserve, memRate, burnMemMode)
}

func runBurnMem(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string) int {
	args := fmt.Sprintf(`%s --nohup --mem-percent %d --reserve %d --rate %d --mode %s`,
		path.Join(util.GetProgramPath(), burnMemBin), memPercent, memReserve, memRate, burnMemMode)

	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := channel.Run(ctx, "nohup", args)
	if !response.Success {
		stopBurnMemFunc()
		bin.PrintErrAndExit(response.Err)
	}
	return -1
}

func stopBurnMem() (success bool, errs string) {
	ctx := context.WithValue(context.Background(), cl.ProcessKey, "nohup")
	pids, _ := cl.GetPidsByProcessName(burnMemBin, ctx)
	var response *spec.Response
	if pids != nil && len(pids) != 0 {
		response = channel.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
		if !response.Success {
			return false, response.Err
		}
	}
	if burnMemMode == "cache" {
		dirPath := path.Join(util.GetProgramPath(), dirName)
		if _, err := os.Stat(dirPath); err == nil {
			response = channel.Run(ctx, "umount", dirPath)

			if !response.Success {
				bin.PrintErrAndExit(response.Error())
			}

			err = os.RemoveAll(dirPath)
			if err != nil {
				bin.PrintErrAndExit(err.Error())
			}
		}
	}
	return true, errs
}

func calculateMemSize(percent, reserve int) (int64, int64, error) {
	mc := cgroups.NewMemory("/sys/fs/cgroup", cgroups.IgnoreModules("memsw"))
	stats := v1.Metrics{}
	if err := mc.Stat("", &stats); err != nil {
		return 0, 0, err
	}

	total := int64(0)
	available := int64(0)
	//no limit
	if stats.Memory.Usage.Limit == PAGE_COUNTER_MAX {
		virtualMemory, err := mem.VirtualMemory()
		if err != nil {
			return 0, 0, err
		}
		total = int64(virtualMemory.Total)
		available = int64(virtualMemory.Available)
	} else {
		total = int64(stats.Memory.Usage.Limit)
		available = int64(stats.Memory.Usage.Limit - stats.Memory.Usage.Usage)
	}

	expectSize := int64(0)
	if percent != 0 {
		expectSize = (available - total*int64(100-percent)/100) / 1024 / 1024
	} else {
		expectSize = available/1024/1024 - int64(reserve)
	}

	return total / 1024 / 1024, expectSize, nil
}
