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
	os_exec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"math/rand"
	"math/big"
	"math"
	"unsafe"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"

	"github.com/mjibson/go-dsp/fft"
	"github.com/shirou/gopsutil/cpu"
	"github.com/howeyc/crc16"
	_ "go.uber.org/automaxprocs/maxprocs"
)

const BurnCpuBin = "chaos_burncpu"

type StressCpuMethodInfo struct {
	name  			string					/* human readable form of stressor */
	stress 			func(context.Context)	/* the cpu method function */
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
		// if cpu-list value is not empty, then the cpu-count flag is invalid.
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

// start burn cpu.
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

	// The default is zero. Whatever the value of percpu, the 
	// cpuIndex can be used as a subscript for totalCpuPercent in burn.
	var cpuIndex int = 0
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

	quotas := make([]chan int64, cpuCount)
	for i := 0; i < cpuCount; i++ {
		quotas[i] = make(chan int64)
		go burn(ctx, quotas[i], slopePercent, precpu, cpuIndex, cpuCount)
	}

	// A percpu of true gets the load of the specified cpu; 
	// A percpu of false gets the load of the average cpu;
	// The two cases are combined in a single loop.
	for {
		q := getQuota(ctx, slopePercent, precpu, cpuIndex)
		for i := 0; i < cpuCount; i++ {
			quotas[i] <- q
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

// If percpu is true, it returns the ratio of the specified cpu-index;
// otherwise, it returns the average cpu load ratio.
func getQuota(ctx context.Context, slopePercent float64, precpu bool, cpuIndex int) int64 {
	used := getUsed(ctx, precpu, cpuIndex)
	log.Debugf(ctx, "cpu usage: %f , precpu: %v, cpuIndex %d", used, precpu, cpuIndex)
	dx := (slopePercent - used) / 100
	busy := int64(dx * float64(period))
	return busy
}

func burn(ctx context.Context, quota <-chan int64, slopePercent float64, precpu bool, cpuIndex int, cpuCount int) {
	var beforeCpuPercent float64 = slopePercent
	cpuNum := runtime.NumCPU()
	if precpu {
		cpuNum = 1
	}
	q := getQuota(ctx, slopePercent, precpu, cpuIndex)
	cpu.Percent(0, false)
	ds := period - q
	if ds < 0 {
		ds = 0
	}
	for {
		select {
		case offset := <-quota:
			q = q + offset
			if q < 0 {
				q = 0
			}
			ds = period - q
			if ds < 0 {
				ds = 0
			}
		default:
			cpuPercent := float64(q)/float64(q+ds)*100
			// This loop is used to handle the update of the quota error q. 
			// Assuming a cpu_percent of 70 and a current system load of 10%, 
			// there is one condition that would make entering this loop:
			// q Execute quota once, then default, then quota, and offset is 60 both times.
			if cpuPercent > slopePercent {
				// precpu is true, get the corresponding index frequency; 
				// otherwise get the average frequency
				totalCpuPercent, err := cpu.Percent(0, precpu)
				if err != nil {
					log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
					continue
				}
				// Here is actually a retry strategy, because 
				// `if cpuPercent > slopePercent` is actually to prevent quota from being inaccurate.
				if totalCpuPercent[cpuIndex] >= slopePercent {
					// 1. A quota is not inaccurate, then do not modify q, ds, directly retry.
					// 2. There are indeed other processes that are hogging the CPU, q, ds will be corrected by quota.
					log.Debugf(ctx, "current CPU load is higher than slopePercent.")
					continue
				}
				// Start calculating q and ds based on totalCpuPercent. 
				// beforeCpuPercent is initially set to slopePercent, 
				// which may be inaccurate and cause a higher load 
				// when there are other processes hogging the CPU.

				// The repaired beforeCpuPercent: cpuCount * beforeCpuPercent / cpuNum
				// beforeCpuPercent ├── cpu-count: Average of cpuCount goroutinue over cpuNum cores.
				// 					└── cpu-index: cpuCount is always equal to 1,cpuNum is always equal to 1.
				other := totalCpuPercent[cpuIndex] - beforeCpuPercent*float64(cpuCount/cpuNum)
				if other < 0 {
					other = 0
				}
				cpuPercent := slopePercent - other
				if cpuPercent < 0 {
					cpuPercent = 0
				}
				q = int64(cpuPercent/float64(100)*float64(period))
				ds = period - q
			}

			// When cpuPercent is zero stress_cpu will call sleep.
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

// TODO: Extend richer CPU burn algorithms:
// floatconversion, gamma, gcd, gray, hamming, hyperbolic, idct...
var cpu_methods = []StressCpuMethodInfo {
	{ "ackermann", 	stress_cpu_ackermann,	},
	{ "bitops",		stress_cpu_bitops,		},
	{ "collatz",	stress_cpu_collatz,		},
	{ "crc16",		stress_cpu_crc16,		},
	{ "factorial",	stress_cpu_factorial,	},
	{ "fft", 		stress_cpu_fft,         },
	{ "pi", 		stress_cpu_pi,			}, 
	{ "fibonacci",	stress_cpu_fibonacci,	},
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

func stress_cpu_ackermann(ctx context.Context) {
	a := ackermann(3, 7)

	if a != 0x3fd {
		log.Fatalf(ctx, "ackermann error detected, ackermann(3,9) miscalculated\n")
	}
}

func stress_cpu_bitops(ctx context.Context) {
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
		log.Fatalf(ctx, "bitops error detected, failed bitops operations\n")
	}
}

func stress_cpu_collatz(ctx context.Context) {
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
		log.Fatalf(ctx, "error detected, failed collatz progression\n")
	}
}

func stress_cpu_crc16(ctx context.Context) {
	var randomBuffer [4096]byte
	rand.Read(randomBuffer[:])
	for i := 0; i < 8; i++ {
		crc16.ChecksumIBM(randomBuffer[:])
	}
}

func stress_cpu_factorial(ctx context.Context) {
	var f float64 = 1.0
	var precision float64 = 1.0e-6

	for n := 1; n < 150; n++ {
		np1 := float64(n + 1)
		fact := math.Round(math.Exp(math.Gamma(np1)))
		var dn float64

		f *= float64(n)

		/* Stirling */
		if (f - fact) / fact > precision {
			log.Fatalf(ctx, "Stirling's approximation of factorial(%d) out of range\n", n)
		}

		/* Ramanujan */
		dn = float64(n)
		fact = math.SqrtPi * math.Pow((dn / float64(math.E)), dn)
		fact *= math.Pow((((((((8 * dn) + 4)) * dn) + 1) * dn) + 1.0/30.0), (1.0/6.0))
		if ((f - fact) / fact > precision) {
			log.Fatalf(ctx, "Stirling's approximation of factorial(%d) out of range\n", n)
		}
	}
}

func stress_cpu_fft(ctx context.Context) {
	var buffer [128]float64
	for i := 0; i < 128; i++ {
		buffer[i] = float64(i%64)
	}
	for i := 0; i < 8; i++ {
		fft.FFTReal(buffer[:])
	}
}

// We start out by defining a high-precision arc cotangent function.  
// This one returns the response as an integer- normally it would be 
// a floating point number.  Here,the integer is multiplied by the 
// "unity" that we pass in. If unity is 10, for example, and the answer 
// should be "0.5",then the answer will come out as 5.
// https://go.dev/play/p/hF9jklt5lp
func stress_cpu_pi(ctx context.Context) {
	digits := big.NewInt(1000)
	unity := big.NewInt(0)
	unity.Exp(big.NewInt(10), digits, nil)
	pi := big.NewInt(0)
	four := big.NewInt(4)
	pi.Mul(four, pi.Sub(pi.Mul(four, arccot(5, unity)), arccot(239, unity)))
}

func arccot(x int64, unity *big.Int) *big.Int {
	bigx := big.NewInt(x)
	xsquared := big.NewInt(x*x)
	sum := big.NewInt(0)
	sum.Div(unity, bigx)
	xpower := big.NewInt(0)
	xpower.Set(sum)
	n := int64(3)
	zero := big.NewInt(0)
	sign := false
	
	term := big.NewInt(0)
	for {
		xpower.Div(xpower, xsquared)
		term.Div(xpower, big.NewInt(n))
		if term.Cmp(zero) == 0 {
			break
		}
		if sign {
			sum.Add(sum, term)
		} else {
			sum.Sub(sum, term)
		}
		sign = !sign
		n += 2
	}
	return sum
}

func stress_cpu_fibonacci(ctx context.Context) {
	var fn_res uint64 = 0xa94fad42221f2702
	var f1 uint64 = 1
	var f2 uint64 = 1
	var fn uint64 = 1

	for !(fn & 0x8000000000000000 != 0) {
		fn = f1 + f2
		f1 = f2
		f2 = fn
	}

	if fn_res != fn {
		log.Fatalf(ctx, "fibonacci error detected, summation or assignment failure\n")
	}
}

// Make a single CPU load rate reach cpuPercent% within the time interval.
// This function can also be used to implement something similar to stress-ng --cpu-load.
func stress_cpu(interval time.Duration, cpuPercent float64) {
	if cpuPercent == 0 {
		time.Sleep(time.Duration(interval)*time.Nanosecond)
		return
	}
	bias := 0.0
	startTime := time.Now().UnixNano()
	nanoInterval := int64(interval/time.Nanosecond)
	for {
		if time.Now().UnixNano() - startTime > nanoInterval {
			break
		}

		startTime1 := time.Now().UnixNano()
		// TODO: The total number of loops and the method can be specified later.
		for i := 0; i < len(cpu_methods); i++ {
			stress_cpu_method(i)
		}
		endTime1 := time.Now().UnixNano()
		delay := ((100 - cpuPercent) * float64(endTime1 - startTime1) / cpuPercent)
		delay -= bias
		if delay <= 0.0 {
			bias = 0.0
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
	cpu_methods[method].stress(context.Background())
}