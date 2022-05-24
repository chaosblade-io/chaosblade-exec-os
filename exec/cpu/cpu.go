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
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"os"
	os_exec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	//"math/rand"
	"math"
	"unsafe"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/shirou/gopsutil/cpu"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	_ "go.uber.org/automaxprocs/maxprocs"
)

const BurnCpuBin = "chaos_burncpu"

type StressCpuMethodInfo struct {
	name  			string			/* human readable form of stressor */
	stress 			func(string) 	/* the cpu method function */
}

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

	var cpuCount int
	var cpuList string
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
			log.Errorf(ctx, "`%s`: cpu-list is illegal, it must be a positive integer and not bigger than 100", cpuPercentStr)
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
		if cpuCount <= 0 || cpuCount > runtime.NumCPU() {
			cpuCount = runtime.NumCPU()
		}
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

	return ce.start(ctx, cpuList, cpuCount, cpuPercent, climbTime, model.ActionFlags["cpu-index"])
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
			command := os_exec.CommandContext(ctx, "taskset", argsArray...)
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
	precpu := false
	if cpuIndexStr != "" {
		precpu = true
		var err error
		cpuIndex, err = strconv.Atoi(cpuIndexStr)
		if err != nil {
			log.Errorf(ctx, "`%s`: cpu-index is illegal, cpu-index value must be a positive integer", cpuIndexStr)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "cpu-index", cpuIndexStr, "it must be a positive integer")
		}
	}

	slope(ctx, cpuPercent, climbTime, slopePercent, precpu, cpuIndex)

	quota := make(chan int64, cpuCount)
	fmt.Printf("cpucount : %d\n", cpuCount)
	for i := 0; i < cpuCount; i++ {
		go burn(ctx, quota, slopePercent, precpu, cpuIndex)
	}

	for {
		q := getQuota(ctx, slopePercent, precpu, cpuIndex)
		for i := 0; i < cpuCount; i++ {
			quota <- q
		}
	}
}

const period = int64(1000000000)

func slope(ctx context.Context, cpuPercent int, climbTime int, slopePercent float64, precpu bool, cpuIndex int) {
	if climbTime != 0 {
		var ticker = time.NewTicker(time.Second)
		slopePercent = getUsed(ctx, precpu, cpuIndex)
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

func getQuota(ctx context.Context, slopePercent float64, precpu bool, cpuIndex int) int64 {
	used := getUsed(ctx, precpu, cpuIndex)
	log.Debugf(ctx, "cpu usage: %f , precpu: %v, cpuIndex %d", used, precpu, cpuIndex)
	dx := (slopePercent - used) / 100
	busy := int64(dx * float64(period))
	fmt.Println("((((((((((((", used, dx, busy, cpuIndex)
	return busy
}

// The root cause of the complexity is that getUsed requires sleep.
func burn(ctx context.Context, quota <-chan int64, slopePercent float64, precpu bool, cpuIndex int) {
	var beforeCpuPercent float64 = 0
	q := getQuota(ctx, slopePercent, precpu, cpuIndex)
	cpu.Percent(time.Second, true)
	ds := period - q
	if ds < 0 {
		ds = 0
	}
	fmt.Println(q, ds, slopePercent)
	for {
		select {
		case offset := <-quota:
			q = q + offset
			if q < 0 {
				q = 0
			}
			ds = period - q
			fmt.Println("////////////", q, ds, offset)
			if ds < 0 {
				ds = 0
			}
		default:
			cpuPercent := float64(q)/float64(q+ds)*100
			// When the first case executes with a lag, it causes q 
			// to be increased by multiple offsets, resulting in a 
			// higher CPU load than expectedPercent.
			// cpuPercent cannot be less than zero.
			fmt.Println("+++++++++++", cpuPercent)
			if cpuPercent == 0 || cpuPercent > slopePercent {
				totalCpuPercent, err := cpu.Percent(0, true)
				if err != nil {
					log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
					continue
				}
				if totalCpuPercent[cpuIndex] >= slopePercent {
					fmt.Println("current CPU load is higher than slopePercent.")
					log.Debugf(ctx, "current CPU load is higher than slopePercent.")
					// When the specified CPU frequency is greater than the expected CPU 
					// frequency of chaos_os, we expect the behavior to be that chaos_os 
					// does not occupy the CPU.
					time.Sleep(time.Second)
					cpu.Percent(time.Second, true)
					continue
				}
				other := totalCpuPercent[cpuIndex] - beforeCpuPercent
				if other < 0 {
					other = 0
				}
				cpuPercent := slopePercent - other
				if cpuPercent < 0 {
					cpuPercent = 0
				}
				q = int64(cpuPercent/float64(100)*float64(period))
				ds = period - q

				fmt.Println("xiufu: ", q, ds, cpuPercent, slopePercent, totalCpuPercent[cpuIndex])
			}
			fmt.Println("------------", q, ds, cpuPercent, float64(q)/float64(q+ds)*100)
			stress_cpu(time.Duration(q+ds), cpuPercent)
			beforeCpuPercent = cpuPercent
		}
	}
}

// stop burn cpu
func (ce *cpuExecutor) stop(ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", BurnCpuBin)
	return exec.Destroy(ctx, ce.channel, "cpu fullload")
}

var cpu_methods = []StressCpuMethodInfo {
	{ "ackermann", 	stress_cpu_ackermann,	},
	{ "bitops",		stress_cpu_bitops,		},
	{ "collatz",	stress_cpu_collatz,		},
	// { "crc16",		stress_cpu_crc16,		},
	{ "factorial",	stress_cpu_factorial,	},
}

func ackermann(m uint32, n uint32) uint32 {
	if m == 0 {
		return n + 1
	} else if n == 0 {
		return ackermann(m - 1, 1)
	} else {
		return ackermann(m - 1, ackermann(m, n - 1))
	}
}

func stress_cpu_ackermann(name string) {
	a := ackermann(3, 7);

	if a != 0x3fd {
		fmt.Printf("%s: ackermann error detected, ackermann(3,9) miscalculated\n", name);
	}
}

func stress_cpu_bitops(name string) {
	var i_sum uint32 = 0
	var sum uint32 = 0x8aac0aab

	for i := 0; i < 16384; i++ {
		{
			var r uint32 = uint32(i)
			var v uint32 = uint32(i)
			var s uint32 = uint32((unsafe.Sizeof(v) * 8) - 1)
			for v >>= 1; v != 0; v, s = v>>1, s-1 {
				r <<= 1
				r |= v & 1
			}
			r <<= s
			i_sum += r
		}
		{
			/* parity check */
			var v uint32 = uint32(i)

			v ^= v >> 16
			v ^= v >> 8
			v ^= v >> 4
			v &= 0xf
			i_sum += (0x6996 >> v) & 1
		}
		{
			/* Brian Kernighan count bits */
			var v uint32 = uint32(i)
			var j uint32 = uint32(i)

			for j = 0; v != 0; j++ {
				v &= v - 1
			}
			i_sum += j
		}
		{
			/* round up to nearest highest power of 2 */
			var v uint32 = uint32(i - 1)

			v |= v >> 1
			v |= v >> 2
			v |= v >> 4
			v |= v >> 8
			v |= v >> 16
			i_sum += v
		}
	}
	if i_sum != sum {
		fmt.Printf("%s: bitops error detected, failed bitops operations\n", name)
	}
}

func stress_cpu_collatz(name string) {
	var n uint64 = 989345275647
	var i int
	for i = 0; n != 1; i++ {
		if n&1 != 0 {
			n = (3 * n) + 1
		} else {
			n = n / 2
		}
	}

	if i != 1348 {
		fmt.Printf("%s: error detected, failed collatz progression\n", name)
	}
}

// func stress_cpu_crc16(name string) {
// 	randBytes := make([]byte, 1024)
// 	rand.Read(randBytes)

// 	for i := 1; i < len(randBytes); i++ {
// 		ccitt_crc16([]uint8(randBytes), i)
// 	}
// }

// func ccitt_crc16(data *uint8, n int) uint16 {
// 	/*
// 	 *  The CCITT CRC16 polynomial is
// 	 *     16    12    5
// 	 *    x   + x   + x  + 1
// 	 *
// 	 *  which is 0x11021, but to make the computation
// 	 *  simpler, this has been reversed to 0x8408 and
// 	 *  the top bit ignored..
// 	 *  We can get away with a 17 bit polynomial
// 	 *  being represented by a 16 bit value because
// 	 *  we are assuming the top bit is always set.
// 	 */
// 	var polynomial uint16 = 0x8408
// 	var crc uint16 = 0xffff

// 	if n == 0 {
// 		return 0
// 	}

// 	for ; n!=0; n-- {
// 		data += 1
// 		var val uint8 = (uint16(0xff) & *data)
// 		for i := 8; i; i, val = i-1, val>>1 {
// 			var do_xor bool = 1 & (val ^ crc)
// 			crc >>= 1;
// 			var tmp uint16 = 0
// 			if do_xor {
// 				tmp = polynomial
// 			}
// 			crc ^= tmp
// 		}
// 	}

// 	crc = ^crc
// 	return ((uint16)(crc << 8)) | (crc >> 8);
// }

func stress_cpu_factorial(name string) {
	var f float64 = 1.0
	var precision float64 = 1.0e-6

	for n := 1; n < 150; n++ {
		np1 := float64(n + 1)
		fact := math.Round(math.Exp(math.Gamma(np1)))
		var dn float64

		f *= float64(n);

		/* Stirling */
		if (f - fact) / fact > precision {
			fmt.Println("%s: Stirling's approximation of factorial(%d) out of range\n",
				name, n);
		}

		/* Ramanujan */
		dn = float64(n);
		fact = math.SqrtPi * math.Pow((dn / float64(math.E)), dn)
		fact *= math.Pow((((((((8 * dn) + 4)) * dn) + 1) * dn) + 1.0/30.0), (1.0/6.0));
		if ((f - fact) / fact > precision) {
			fmt.Println("%s: Ramanujan's approximation of factorial(%d) out of range\n",
				name, n);
		}
	}
}

// Make a single CPU load rate reach cpuPercent% within the time interval.
// This function can also be used to implement something similar to stress-ng --cpu-load.
func stress_cpu(interval time.Duration, cpuPercent float64) {
	bias := 0.0
	startTime := time.Now().UnixNano()
	nanoInterval := int64(interval/time.Nanosecond)
	for {
		if time.Now().UnixNano() - startTime > nanoInterval {
			break
		}

		startTime1 := time.Now().UnixNano()
		// Loops and methods may be specified later.
		for i := 0; i < len(cpu_methods); i++ {
			stress_cpu_method(i)
		}
		endTime1 := time.Now().UnixNano()
		//fmt.Println(startTime1, endTime1, cpuPercent)
		delay := ((100 - cpuPercent) * float64(endTime1 - startTime1) / cpuPercent)
		//fmt.Printf("delay : [%f], bias : [%f]\n", delay, bias)
		delay -= bias
		if delay <= 0.0 {
			bias = 0.0;
		} else {
			startTime2 := time.Now().UnixNano()
			time.Sleep(time.Duration(delay) * time.Nanosecond)
			endTime2 := time.Now().UnixNano()
			bias = float64(endTime2 - startTime2) - delay
		}
	}
}

// No need to perform subscript checking.
func stress_cpu_method(method int) {
	cpu_methods[method].stress("lizhaolong");
}