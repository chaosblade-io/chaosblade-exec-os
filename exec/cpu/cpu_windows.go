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
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/shirou/gopsutil/process"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/shirou/gopsutil/cpu"

	_ "go.uber.org/automaxprocs/maxprocs"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const BurnCpuBin = "chaos_burncpu"

type CpuCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewCpuCommandModelSpec() spec.ExpModelCommandSpec {
	return &CpuCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				&FullLoadActionCommand{
					spec.BaseExpActionCommandSpec{
						ActionMatchers: []spec.ExpFlagSpec{},
						ActionFlags:    []spec.ExpFlagSpec{},
						ActionExecutor: &cpuExecutor{},
						ActionExample: `
# Create a CPU full load experiment
blade create cpu load

#Specifies two random core's full load
blade create cpu load --cpu-percent 60 --cpu-count 2

# Specified percentage load
blade create cpu load --cpu-percent 60`,
						ActionPrograms:    []string{BurnCpuBin},
						ActionCategories:  []string{category.SystemCpu},
						ActionProcessHang: true,
					},
				},
			},
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "cpu-count",
					Desc:     "Cpu count",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "cpu-percent",
					Desc:     "percent of burn CPU (0-100)",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "climb-time",
					Desc:     "durations(s) to climb",
					Required: false,
				},
			},
		},
	}
}

func (*CpuCommandModelSpec) Name() string {
	return "cpu"
}

func (*CpuCommandModelSpec) ShortDesc() string {
	return "Cpu experiment"
}

func (*CpuCommandModelSpec) LongDesc() string {
	return "Cpu experiment, for example full load"
}

type FullLoadActionCommand struct {
	spec.BaseExpActionCommandSpec
}

func (*FullLoadActionCommand) Name() string {
	return "fullload"
}

func (*FullLoadActionCommand) Aliases() []string {
	return []string{"fl", "load"}
}

func (*FullLoadActionCommand) ShortDesc() string {
	return "cpu load"
}

func (f *FullLoadActionCommand) LongDesc() string {
	if f.ActionLongDesc != "" {
		return f.ActionLongDesc
	}
	return "Create chaos engineering experiments with CPU load"
}

func (*FullLoadActionCommand) Matchers() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

func (*FullLoadActionCommand) Flags() []spec.ExpFlagSpec {
	return []spec.ExpFlagSpec{}
}

type cpuExecutor struct {
	channel spec.Channel
}

func (ce *cpuExecutor) Name() string {
	return "cpu"
}

func (ce *cpuExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}

var slopePercent float64

func (ce *cpuExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if ce.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx)
	}

	var cpuCount int
	var cpuPercent int
	var climbTime int

	cpuPercentStr := model.ActionFlags["cpu-percent"]
	if cpuPercentStr != "" {
		var err error
		cpuPercent, err = strconv.Atoi(cpuPercentStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: cpu-percent is illegal, it must be a positive integer", cpuPercentStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-percent", cpuPercentStr, "it must be a positive integer")
		}
		if cpuPercent > 100 || cpuPercent < 0 {
			log.Errorf(ctx, "`%s`: cpu-percent is illegal, it must be a positive integer and not bigger than 100", cpuPercentStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-percent", cpuPercentStr, "it must be a positive integer and not bigger than 100")
		}
	} else {
		cpuPercent = 100
	}

	var err error
	cpuCountStr := model.ActionFlags["cpu-count"]
	if cpuCountStr != "" {
		cpuCount, err = strconv.Atoi(cpuCountStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: cpu-count is illegal, cpu-count value must be a positive integer", cpuCountStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-count", cpuCountStr, "it must be a positive integer")
		}
	}
	defaultCpuCount, _ := cpu.Counts(true)
	if cpuCount <= 0 || cpuCount > defaultCpuCount {
		cpuCount = defaultCpuCount
	}

	climbTimeStr := model.ActionFlags["climb-time"]
	if climbTimeStr != "" {
		var err error
		climbTime, err = strconv.Atoi(climbTimeStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: climb-time is illegal, climb-time value must be a positive integer", climbTimeStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "climb-time", climbTimeStr, "it must be a positive integer")
		}
		if climbTime > 600 || climbTime < 0 {
			log.Errorf(ctx, "`%s`: climb-time is illegal, climb-time value must be a positive integer and not bigger than 600", climbTimeStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "climb-time", climbTimeStr, "must be a positive integer and not bigger than 600")
		}
	}

	return ce.start(ctx, cpuCount, cpuPercent, climbTime)
}

// start burn cpu
func (ce *cpuExecutor) start(ctx context.Context, cpuCount, cpuPercent, climbTime int) *spec.Response {
	runtime.GOMAXPROCS(cpuCount)

	var totalCpuPercent []float64
	var curProcess *process.Process
	var curCpuPercent float64
	var err error

	totalCpuPercent, err = cpu.Percent(time.Second, false)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "get total cpu percent", err.Error())
	}

	curProcess, err = process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "get current pid", err.Error())
	}

	curCpuPercent, err = curProcess.CPUPercent()
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "get pid cpu percent", err.Error())
	}

	otherCpuPercent := (100.0 - (totalCpuPercent[0] - curCpuPercent)) / 100.0
	go func() {
		t := time.NewTicker(3 * time.Second)
		for {
			select {
			// timer 3s
			case <-t.C:
				totalCpuPercent, err = cpu.Percent(time.Second, false)
				if err != nil {
					log.Errorf(ctx, "get total cpu percent failed, err: %v", err)
					return
				}

				curCpuPercent, err = curProcess.CPUPercent()
				if err != nil {
					log.Errorf(ctx, "get current process cpu percent failed, err: %v", err)
					return
				}
				otherCpuPercent = (100.0 - (totalCpuPercent[0] - curCpuPercent)) / 100.0
			}
		}
	}()

	if climbTime == 0 {
		slopePercent = float64(cpuPercent)
	} else {
		var ticker *time.Ticker = time.NewTicker(1 * time.Second)
		slopePercent = totalCpuPercent[0]
		var startPercent = float64(cpuPercent) - slopePercent
		go func() {
			for range ticker.C {
				if slopePercent < float64(cpuPercent) {
					slopePercent += startPercent / float64(climbTime)
				} else if slopePercent > float64(cpuPercent) {
					slopePercent -= startPercent / float64(climbTime)
				}
			}
		}()
	}

	for i := 0; i < cpuCount; i++ {
		go func() {
			busy := int64(0)
			idle := int64(0)
			all := int64(10000000)
			dx := 0.0
			ds := time.Duration(0)
			for i := 0; ; i = (i + 1) % 1000 {
				startTime := time.Now().UnixNano()
				if i == 0 {
					dx = (slopePercent - totalCpuPercent[0]) / otherCpuPercent
					busy = busy + int64(dx*100000)
					if busy < 0 {
						busy = 0
					}
					idle = all - busy
					if idle < 0 {
						idle = 0
					}
					ds, _ = time.ParseDuration(strconv.FormatInt(idle, 10) + "ns")
				}
				for time.Now().UnixNano()-startTime < busy {
				}
				time.Sleep(ds)
				runtime.Gosched()
			}
		}()
	}
	select {}
}

// stop burn cpu
func (ce *cpuExecutor) stop(ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", BurnCpuBin)
	return exec.Destroy(ctx, ce.channel, "cpu fullload")
}
