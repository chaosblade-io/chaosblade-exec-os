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

package burncpu

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_startBurnCpu(t *testing.T) {
	type args struct {
		cpuList    string
		cpuCount   int
		cpuPercent int
	}
	tests := []struct {
		name string
		args args
	}{
		{"test1", args{"1,2,3,5", 0, 50}},
		{"test2", args{"", 3, 50}},
	}

	burnCPU := &BurnCPU{}
	burnCPU.Assign()
	burnCPU.Channel = channel.NewMockLocalChannel()
	burnCPU.RunBurnCpu = func(ctx context.Context, cpuCount int, cpuPercent int, pidNeeded bool, processor string, climTime int) int {
		return 25233
	}
	burnCPU.BindBurnCpuByTaskSet = func(ctx context.Context, core string, pid int) {}
	burnCPU.CheckBurnCpu = func(ctx context.Context) {}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			burnCPU.CpuList = tt.args.cpuList
			burnCPU.CpuCount = tt.args.cpuCount
			burnCPU.CpuPercent = tt.args.cpuPercent
			burnCPU.startBurnCpu()
		})
	}
}
func Test_runBurnCpu_failed(t *testing.T) {
	type args struct {
		cpuCount   int
		cpuPercent int
		pidNeeded  bool
		processor  string
	}
	burnBin := path.Join(util.GetProgramPath(), exec.BurnCpuBin)
	as := &args{
		cpuCount:   2,
		cpuPercent: 50,
		pidNeeded:  false,
		processor:  "",
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	burnCPU := &BurnCPU{}
	burnCPU.Assign()
	burnCPU.Channel = channel.NewMockLocalChannel()
	burnCPU.StopBurnCpu = func() (bool, string) {
		return true, ""
	}

	mockChannel := burnCPU.Channel.(*channel.MockLocalChannel)
	actualCommands := make([]string, 0)
	mockChannel.RunFunc = func(ctx context.Context, script, args string) *spec.Response {
		actualCommands = append(actualCommands, fmt.Sprintf("%s %s", script, args))
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], "nohup command not found")
	}
	expectedCommands := []string{fmt.Sprintf(`nohup %s --nohup --cpu-count 2 --cpu-percent 50 > /dev/null 2>&1 &`, burnBin)}

	burnCPU.RunBurnCpu(context.Background(), as.cpuCount, as.cpuPercent, as.pidNeeded, as.processor, 0)
	if exitCode != 1 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 1)
	}
	if !reflect.DeepEqual(expectedCommands, actualCommands) {
		t.Errorf("unexpected commands: %+v, expected commands: %+v", actualCommands, expectedCommands)
	}
}

func Test_bindBurnCpu(t *testing.T) {
	type args struct {
		core string
		pid  int
	}
	as := &args{
		core: "0",
		pid:  25233,
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	burnCPU := &BurnCPU{}
	burnCPU.Assign()
	burnCPU.Channel = channel.NewMockLocalChannel()
	burnCPU.StopBurnCpu = func() (bool, string) { return true, "" }
	mockChannel := burnCPU.Channel.(*channel.MockLocalChannel)
	actualCommands := make([]string, 0)
	mockChannel.RunFunc = func(ctx context.Context, script, args string) *spec.Response {
		actualCommands = append(actualCommands, fmt.Sprintf("%s %s", script, args))
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], "taskset command not found")
	}
	expectedCommands := []string{fmt.Sprintf(`taskset -a -cp 0 25233`)}

	burnCPU.BindBurnCpuByTaskSet(context.Background(), as.core, as.pid)
	if exitCode != 1 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 1)
	}
	if !reflect.DeepEqual(expectedCommands, actualCommands) {
		t.Errorf("unexpected commands: %+v, expected commands: %+v", actualCommands, expectedCommands)
	}
}
func Test_checkBurnCpu(t *testing.T) {
	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	burnCPU := &BurnCPU{}
	burnCPU.Assign()
	burnCPU.Channel = channel.NewMockLocalChannel()
	burnCPU.CheckBurnCpu(context.Background())
	if exitCode != 1 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}
}

func Test_stopBurnCpu(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"stop"},
	}

	burnCPU := &BurnCPU{}
	burnCPU.Assign()
	burnCPU.Channel = channel.NewMockLocalChannel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			burnCPU.StopBurnCpu()
		})
	}
}
