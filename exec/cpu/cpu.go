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
	"github.com/sirupsen/logrus"
	"os"
	os_exec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	_ "go.uber.org/automaxprocs/maxprocs"
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

# Specifies that the core is full load with index 0, 3, and that the core's index starts at 0
blade create cpu load --cpu-list 0,3

# Specify the core full load of indexes 1-3
blade create cpu load --cpu-list 1-3

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
					Name:     "cpu-list",
					Desc:     "CPUs in which to allow burning (0-3 or 1,3)",
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

func (ce *cpuExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if ce.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx)
	}

	var cpuCount int
	var cpuList string
	var cpuPercent int
	var climbTime int

	cpuPercentStr := model.ActionFlags["cpu-percent"]
	if cpuPercentStr != "" {
		var err error
		cpuPercent, err = strconv.Atoi(cpuPercentStr)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: cpu-percent is illegal, it must be a positive integer", cpuPercentStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-percent", cpuPercentStr, "it must be a positive integer")
		}
		if cpuPercent > 100 || cpuPercent < 0 {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: cpu-list is illegal, it must be a positive integer and not bigger than 100", cpuPercentStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-percent", cpuPercentStr, "it must be a positive integer and not bigger than 100")
		}
	} else {
		cpuPercent = 100
	}

	cpuListStr := model.ActionFlags["cpu-list"]
	if cpuListStr != "" {
		if !ce.channel.IsCommandAvailable(ctx, "taskset") {
			return spec.ResponseFailWithFlags(spec.CommandTasksetNotFound)
		}
		cores, err := util.ParseIntegerListToStringSlice("cpu-list", cpuListStr)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: cpu-list is illegal, %s", cpuListStr, err.Error()))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-list", cpuListStr, err.Error())
		}
		cpuList = strings.Join(cores, ",")
	} else {
		// if cpu-list value is not empty, then the cpu-count flag is invalid
		var err error
		cpuCountStr := model.ActionFlags["cpu-count"]
		if cpuCountStr != "" {
			cpuCount, err = strconv.Atoi(cpuCountStr)
			if err != nil {
				util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: cpu-count is illegal, cpu-count value must be a positive integer", cpuCountStr))
				return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-count", cpuCountStr, "it must be a positive integer")
			}
		}
		if cpuCount <= 0 || int(cpuCount) > runtime.NumCPU() {
			cpuCount = runtime.NumCPU()
		}
	}

	climbTimeStr := model.ActionFlags["climb-time"]
	if climbTimeStr != "" {
		var err error
		climbTime, err = strconv.Atoi(climbTimeStr)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: climb-time is illegal, climb-time value must be a positive integer", climbTimeStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "climb-time", climbTimeStr, "it must be a positive integer")
		}
		if climbTime > 600 || climbTime < 0 {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: climb-time is illegal, climb-time value must be a positive integer and not bigger than 600", climbTimeStr))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "climb-time", climbTimeStr, "must be a positive integer and not bigger than 600")
		}
	}

	return ce.start(ctx, cpuList, cpuCount, cpuPercent, climbTime)
}

// start burn cpu
func (ce *cpuExecutor) start(ctx context.Context, cpuList string, cpuCount int, cpuPercent int, climbTime int) *spec.Response {
	ce.channel.GetScriptPath()

	if cpuList != "" {
		cores := strings.Split(cpuList, ",")

		for _, core := range cores {

			args := fmt.Sprintf(`%s create cpu fullload --cpu-count 1 --cpu-percent %d --climb-time %d`,
				os.Args[0], cpuPercent, climbTime)

			args = fmt.Sprintf("-c %s %s", core, args)
			argsArray := strings.Split(args, " ")
			command := os_exec.CommandContext(ctx, "taskset", argsArray...)
			command.SysProcAttr = &syscall.SysProcAttr{}

			if err := command.Start(); err != nil {
				sprintf := fmt.Sprintf("taskset exec failed, %v", err)
				return spec.ReturnFail(spec.OsCmdExecFailed, sprintf)
			}
		}
	}

	runtime.GOMAXPROCS(cpuCount)
	logrus.Debugf("cpu counts: %d", cpuCount)
	slopePercent := float64(cpuPercent)
	slope(ctx, cpuPercent, climbTime, slopePercent, cpuCount)

	quota := make(chan int64)
	for i := 0; i < cpuCount; i++ {
		go burn(ctx, quota, slopePercent, cpuCount)
		go func() {
			for {
				quota <- getQuota(ctx, slopePercent, cpuCount)
			}
		}()
	}

	for {
		used := getUsed(ctx, cpuCount)
		logrus.Debugf("used: %f \n", used)
	}
}

const period = int64(10000000)

func slope(ctx context.Context, cpuPercent int, climbTime int, slopePercent float64, cpuCount int) {
	if climbTime != 0 {
		var ticker = time.NewTicker(time.Second)
		slopePercent = getUsed(ctx, cpuPercent)
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
}

func getQuota(ctx context.Context, slopePercent float64, cpuCount int) int64 {
	used := getUsed(ctx, cpuCount)
	dx := 0.0
	if used > 100 {
		dx = (slopePercent - used) / 100
	} else {
		dx = (slopePercent - used) / (100 - used)
	}
	busy := int64(dx * float64(period))
	return busy
}

func burn(ctx context.Context, quota <-chan int64, slopePercent float64, cpuCount int) {
	q := getQuota(ctx, slopePercent, cpuCount)
	ds := period - q
	if ds < 0 {
		ds = 0
	}
	s, _ := time.ParseDuration(strconv.FormatInt(ds, 10) + "ns")
	for {
		select {
		case offset := <-quota:
			q = q + offset
			if q < 0 {
				q = 0
			}
			ds := period - q
			if ds < 0 {
				ds = 0
			}
			logrus.Debug(q, ds)
			s, _ = time.ParseDuration(strconv.FormatInt(ds, 10) + "ns")
		default:
			startTime := time.Now().UnixNano()
			for time.Now().UnixNano()-startTime < q {
			}
			time.Sleep(s)
			runtime.Gosched()
		}
	}
}

// stop burn cpu
func (ce *cpuExecutor) stop(ctx context.Context) *spec.Response {
	return exec.Destroy(ctx, ce.channel, "cpu fullload")
}
