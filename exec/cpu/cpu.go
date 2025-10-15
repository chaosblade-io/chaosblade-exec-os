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
	"math"
	"os"
	osexec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-exec-os/pkg/automaxprocs"

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
					Name:     "cpu-index",
					Desc:     "cpu index, user unavailable!",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "climb-time",
					Desc:     "durations(s) to climb",
					Required: false,
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

	var (
		cpuCount   int
		cpuList    string
		cpuPercent int
		climbTime  int
		quotaRatio = 1.0
	)

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

	cpuListStr := model.ActionFlags["cpu-list"]
	if cpuListStr != "" {
		if !ce.channel.IsCommandAvailable(ctx, "taskset") {
			return spec.ResponseFailWithFlags(spec.CommandTasksetNotFound)
		}
		cores, err := util.ParseIntegerListToStringSlice("cpu-list", cpuListStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: cpu-list is illegal, %s", cpuListStr, err.Error())
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
				log.Errorf(ctx, "`%s`: cpu-count is illegal, cpu-count value must be a positive integer", cpuCountStr)
				return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-count", cpuCountStr, "it must be a positive integer")
			}
		}

		tmpCpuCnt := runtime.NumCPU()
		tmpQuotaRatio := 1.0
		if _, ok := ce.channel.(*channel.NSExecChannel); ok {
			tmpCpuCnt, tmpQuotaRatio, err = automaxprocs.GetCPUCntByPid(
				ctx,
				model.ActionFlags["cgroup-root"],
				model.ActionFlags[channel.NSTargetFlagName],
			)
			if err != nil {
				log.Errorf(ctx, "get cpu count by pid failed, %v", err)
			}
		}

		if cpuCount <= 0 || cpuCount > tmpCpuCnt {
			cpuCount = tmpCpuCnt
			quotaRatio = tmpQuotaRatio
		}
		log.Infof(ctx, "cpu count: %d, quota ratio: %f", cpuCount, quotaRatio)
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

	ctx = context.WithValue(ctx, "cgroup-root", model.ActionFlags["cgroup-root"])

	// Apply quota ratio to adjust the target percent
	// Example: quota=0.6 cores, cpuCount=1, ratio=0.6
	// User wants 80% load, effective target = 80% * 0.6 = 48%
	// percent should not exceed 100%
	// percent should not be less than 1%, otherwise the cpu burn program will not work
	effectivePercent := int(math.Min(math.Max(float64(cpuPercent)*quotaRatio, 1), 100))
	if effectivePercent != cpuPercent {
		log.Infof(ctx, "adjusted cpu percent from %d%% to %d%% based on quota ratio %f",
			cpuPercent, effectivePercent, quotaRatio)
	}

	return ce.start(ctx, cpuList, cpuCount, effectivePercent, climbTime, model.ActionFlags["cpu-index"])
}

// start burn cpu
func (ce *cpuExecutor) start(ctx context.Context, cpuList string, cpuCount, cpuPercent, climbTime int, cpuIndexStr string) *spec.Response {
	ctx = context.WithValue(ctx, "cpuCount", cpuCount)
	if cpuList != "" {
		cores, err := util.ParseIntegerListToStringSlice("cpu-list", cpuList)
		if err != nil {
			log.Errorf(ctx, "`%s`: cpu-list is illegal, %s", cpuList, err.Error())
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-list", cpuList, err.Error())
		}
		for _, core := range cores {

			args := fmt.Sprintf(`%s create cpu fullload --cpu-count 1 --cpu-percent %d --climb-time %d --cpu-index %s --uid %s`,
				os.Args[0], cpuPercent, climbTime, core, ctx.Value(spec.Uid))

			args = fmt.Sprintf("-c %s %s", core, args)
			argsArray := strings.Split(args, " ")
			command := osexec.CommandContext(ctx, "taskset", argsArray...)
			command.SysProcAttr = &syscall.SysProcAttr{}

			if err := command.Start(); err != nil {
				return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("taskset exec failed, %v", err))
			}
		}
		return spec.ReturnSuccess(ctx.Value(spec.Uid))
	}

	runtime.GOMAXPROCS(cpuCount)
	log.Debugf(ctx, "cpu counts: %d", cpuCount)
	slopePercent := float64(cpuPercent)

	var cpuIndex int
	percpu := false
	if cpuIndexStr != "" {
		percpu = true
		var err error
		cpuIndex, err = strconv.Atoi(cpuIndexStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: cpu-index is illegal, cpu-index value must be a positive integer", cpuIndexStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-index", cpuIndexStr, "it must be a positive integer")
		}
	}

	// make CPU slowly climb to some level, to simulate slow resource competition
	// which system faults cannot be quickly noticed by monitoring system.
	slope(ctx, cpuPercent, climbTime, &slopePercent, percpu, cpuIndex)

	quota := make(chan int64, cpuCount)
	for i := 0; i < cpuCount; i++ {
		go burn(ctx, quota, slopePercent, percpu, cpuIndex)
	}

	for {
		q := getQuota(ctx, slopePercent, percpu, cpuIndex)
		for i := 0; i < cpuCount; i++ {
			quota <- q
		}
	}
}

const period = int64(1000000000)

func slope(ctx context.Context, cpuPercent int, climbTime int, slopePercent *float64, percpu bool, cpuIndex int) {
	if climbTime != 0 {
		ticker := time.NewTicker(time.Second)
		*slopePercent = getUsed(ctx, percpu, cpuIndex)
		startPercent := float64(cpuPercent) - *slopePercent
		go func() {
			for range ticker.C {
				if *slopePercent < float64(cpuPercent) {
					*slopePercent += startPercent / float64(climbTime)
				} else if *slopePercent > float64(cpuPercent) {
					*slopePercent -= startPercent / float64(climbTime)
				}
			}
		}()
	}
}

func getQuota(ctx context.Context, slopePercent float64, percpu bool, cpuIndex int) int64 {
	used := getUsed(ctx, percpu, cpuIndex)
	log.Debugf(ctx, "cpu usage: %f , percpu: %v, cpuIndex %d", used, percpu, cpuIndex)
	dx := (slopePercent - used) / 100
	busy := int64(dx * float64(period))
	return busy
}

func burn(ctx context.Context, quota <-chan int64, slopePercent float64, percpu bool, cpuIndex int) {
	q := getQuota(ctx, slopePercent, percpu, cpuIndex)
	ds := period - q
	if ds < 0 {
		ds = 0
	}
	s, _ := time.ParseDuration(strconv.FormatInt(ds, 10) + "ns")
	for {
		startTime := time.Now().UnixNano()
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
			s, _ = time.ParseDuration(strconv.FormatInt(ds, 10) + "ns")
		default:
			for time.Now().UnixNano()-startTime < q {
			}
			runtime.Gosched()
			time.Sleep(s)
		}
	}
}

// stop burn cpu
func (ce *cpuExecutor) stop(ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", BurnCpuBin)
	return exec.Destroy(ctx, ce.channel, "cpu fullload")
}
