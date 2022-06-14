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
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"io/ioutil"
	"math"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const BurnMemBin = "chaos_burnmem"

type MemCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewMemCommandModelSpec() spec.ExpModelCommandSpec {
	return &MemCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				&MemLoadActionCommand{
					spec.BaseExpActionCommandSpec{
						ActionMatchers: []spec.ExpFlagSpec{},
						ActionFlags:    []spec.ExpFlagSpec{},
						ActionExecutor: &memExecutor{},
						ActionExample: `
# The execution memory footprint is 50%
blade create mem load --mode ram --mem-percent 50

# The execution memory footprint is 50%, cache model
blade create mem load --mode cache --mem-percent 50

# The execution memory footprint is 50%, usage contains buffer/cache
blade create mem load --mode ram --mem-percent 50 --include-buffer-cache

# The execution memory footprint is 50%, avoid mem-burn process being killed
blade create mem load --mode ram --mem-percent 50 --avoid-being-killed

# The execution memory footprint is 50% for 200 seconds
blade create mem load --mode ram --mem-percent 50 --timeout 200

# 200M memory is reserved
blade create mem load --mode ram --reserve 200 --rate 100`,
						ActionPrograms:    []string{BurnMemBin},
						ActionCategories:  []string{category.SystemMem},
						ActionProcessHang: true,
					},
				},
			},
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "mem-percent",
					Desc:     "percent of burn Memory (0-100), must be a positive integer",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "reserve",
					Desc:     "reserve to burn Memory, unit is MB. If the mem-percent flag exist, use mem-percent first.",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "rate",
					Desc:     "burn memory rate, unit is M/S, only support for ram mode.",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "mode",
					Desc:     "burn memory mode, cache or ram.",
					Required: false,
				},
				&spec.ExpFlag{
					Name:   "include-buffer-cache",
					Desc:   "Ram mode mem-percent is include buffer/cache",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "avoid-being-killed",
					Desc:   "Prevent mem-burn process from being killed by oom-killer",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:     "cgroup-root",
					Desc:     "cgroup root path, default value /sys/fs/cgroup",
					NoArgs:   false,
					Required: false,
					Default:  "/sys/fs/cgroup",
				},
			},
		},
	}
}

func (*MemCommandModelSpec) Name() string {
	return "mem"
}

func (*MemCommandModelSpec) ShortDesc() string {
	return "Mem experiment"
}

func (*MemCommandModelSpec) LongDesc() string {
	return "Mem experiment, for example load"
}

func (*MemCommandModelSpec) Example() string {
	return "mem load"
}

type MemLoadActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func (*MemLoadActionCommand) Name() string {
	return "load"
}

func (*MemLoadActionCommand) Aliases() []string {
	return []string{}
}

func (*MemLoadActionCommand) ShortDesc() string {
	return "mem load"
}

func (l *MemLoadActionCommand) LongDesc() string {
	if l.ActionLongDesc != "" {
		return l.ActionLongDesc
	}
	return "Create chaos engineering experiments with memory load"
}

func (*MemLoadActionCommand) Matchers() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

func (*MemLoadActionCommand) Flags() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

type memExecutor struct {
	channel spec.Channel
}

func (ce *memExecutor) Name() string {
	return "mem"
}

func (ce *memExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}

const (
	//processOOMScoreAdj = "/proc/%s/oom_score_adj"
	//oomMinScore        = "-1000"
	processOOMAdj = "/proc/%d/oom_adj"
	oomMinAdj     = "-17"
)

func (ce *memExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"dd", "mount", "umount"}
	if response, ok := ce.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	if ce.channel == nil {
		log.Errorf(ctx, spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx, model.ActionFlags["mode"])
	}
	var memPercent, memReserve, memRate int

	memPercentStr := model.ActionFlags["mem-percent"]
	memReserveStr := model.ActionFlags["reserve"]
	memRateStr := model.ActionFlags["rate"]
	burnMemModeStr := model.ActionFlags["mode"]
	includeBufferCache := model.ActionFlags["include-buffer-cache"] == "true"
	avoidBeingKilled := model.ActionFlags["avoid-being-killed"] == "true"

	var err error
	if memPercentStr != "" {
		var err error
		memPercent, err = strconv.Atoi(memPercentStr)
		if err != nil {
			log.Errorf(ctx,"`%s`: mem-percent  must be a positive integer", memPercentStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mem-percent", memPercentStr, "it must be a positive integer")
		}
		if memPercent > 100 || memPercent < 0 {
			log.Errorf(ctx, "`%s`: mem-percent  must be a positive integer and not bigger than 100", memPercentStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mem-percent", memPercentStr, "it must be a positive integer and not bigger than 100")
		}
	} else if memReserveStr != "" {
		memReserve, err = strconv.Atoi(memReserveStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: reserve  must be a positive integer", memReserveStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "reserve", memReserveStr, err)
		}
	} else {
		memPercent = 100
	}
	if memRateStr != "" {
		memRate, err = strconv.Atoi(memRateStr)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "rate", memRateStr, "it must be a positive integer")
		}
	}
	ctx = context.WithValue(ctx, "cgroup-root", model.ActionFlags["cgroup-root"])
	ce.start(ctx, memPercent, memReserve, memRate, burnMemModeStr, includeBufferCache, avoidBeingKilled, ce.channel)
	return spec.Success()
}

// 128K
type Block [32 * 1024]int32

const PageCounterMax uint64 = 9223372036854770000

func calculateMemSize(ctx context.Context, burnMemMode string, percent, reserve int, includeBufferCache bool) (int64, int64, error) {

	total, available, err := getAvailableAndTotal(ctx, burnMemMode, includeBufferCache)
	if err != nil {
		return 0, 0, err
	}

	reserved := int64(0)
	if percent != 0 {
		reserved = (total * int64(100-percent) / 100) / 1024 / 1024
	} else {
		reserved = int64(reserve)
	}
	expectSize := available/1024/1024 - reserved

	log.Debugf(ctx, "available: %d, percent: %d, reserved: %d, expectSize: %d",
		available/1024/1024, percent, reserved, expectSize)

	return total / 1024 / 1024, expectSize, nil
}

var dirName = "burnmem_tmpfs"

var fileName = "file"

var fileCount = 1

func burnMemWithCache(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string, includeBufferCache bool, cl spec.Channel) {
	filePath := path.Join(path.Join(util.GetProgramPath(), dirName), fileName)
	tick := time.Tick(time.Second)
	for range tick {
		_, expectMem, err := calculateMemSize(ctx, burnMemMode, memPercent, memReserve, includeBufferCache)
		if err != nil {
			log.Fatalf(ctx, "calculate memsize err, %v", err)
		}
		fillMem := expectMem
		if expectMem > 0 {
			if expectMem > int64(memRate) {
				fillMem = int64(memRate)
			}
			nFilePath := fmt.Sprintf("%s%d", filePath, fileCount)
			response := cl.Run(ctx, "dd", fmt.Sprintf("if=/dev/zero of=%s bs=1M count=%d", nFilePath, fillMem))
			if !response.Success {
				log.Fatalf(ctx, "burn mem with cache err, %v", err)
			}
			fileCount++
		}
	}
}

// start burn mem
func (ce *memExecutor) start(ctx context.Context, memPercent, memReserve, memRate int, burnMemMode string, includeBufferCache bool, avoidBeingKilled bool, cl spec.Channel) {
	// adjust process oom_score_adj to avoid being killed
	if avoidBeingKilled {
		scoreAdjFile := fmt.Sprintf(processOOMAdj, os.Getpid())
		if _, err := os.Stat(scoreAdjFile); err == nil || os.IsExist(err)  {
			if err := ioutil.WriteFile(scoreAdjFile, []byte(oomMinAdj), 0644); err != nil {
				log.Errorf(ctx, "run burn memory by %s mode failed, cannot edit the process oom_score_adj, %v", burnMemMode, err)
			}
		} else {
			log.Errorf(ctx, "score adjust file: %s not exists, %v", scoreAdjFile, err)
		}
	}

	if burnMemMode == "cache" {
		burnMemWithCache(ctx, memPercent, memReserve, memRate, burnMemMode, includeBufferCache, cl)
		return
	}
	tick := time.Tick(time.Second)
	var cache = make(map[int][]Block, 1)
	var count = 1
	cache[count] = make([]Block, 0)
	if memRate <= 0 {
		memRate = 100
	}
	for range tick {
		_, expectMem, err := calculateMemSize(ctx, burnMemMode, memPercent, memReserve, includeBufferCache)
		if err != nil {
			log.Fatalf(ctx, "calculate memsize err, %v", err.Error())
		}
		fillMem := expectMem
		if expectMem > 0 {
			if expectMem > int64(memRate) {
				fillMem = int64(memRate)
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
			log.Debugf(ctx, "count: %d, len(buf): %d, cap(buf): %d, expect mem: %d, fill size: %d",
				count, len(buf), cap(buf), expectMem, fillSize)
			cache[count] = append(buf, make([]Block, fillSize)...)
		}
	}
}

// stop burn mem
func (ce *memExecutor) stop(ctx context.Context, burnMemMode string) *spec.Response {
	ctx = context.WithValue(ctx,"bin", BurnMemBin)
	return exec.Destroy(ctx, ce.channel, "mem load")
}
