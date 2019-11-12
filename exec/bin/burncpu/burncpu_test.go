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
	"fmt"
	"path"
	"testing"

	cl "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

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
	runBurnCpuFunc = func(ctx context.Context, cpuCount int, cpuPercent int, pidNeeded bool, processor string) int {
		return 25233
	}
	bindBurnCpuFunc = func(ctx context.Context, core string, pid int) {}
	checkBurnCpuFunc = func(ctx context.Context) {}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuList = tt.args.cpuList
			cpuCount = tt.args.cpuCount
			cpuPercent = tt.args.cpuPercent
			startBurnCpu()
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
	burnBin := path.Join(util.GetProgramPath(), "chaos_burncpu")
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
	var invokeTime int
	stopBurnCpuFunc = func() (bool, string) {
		invokeTime++
		return true, ""
	}

	channel = &cl.MockLocalChannel{
		Response:         spec.ReturnFail(spec.Code[spec.CommandNotFound], "nohup command not found"),
		ExpectedCommands: []string{fmt.Sprintf(`nohup %s --nohup --cpu-count 2 --cpu-percent 50 > /dev/null 2>&1 &`, burnBin)},
		T:                t,
	}

	runBurnCpu(context.Background(), as.cpuCount, as.cpuPercent, as.pidNeeded, as.processor)
	if exitCode != 1 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}
	if invokeTime != 1 {
		t.Errorf("unexpected invoke time %d, expected result: %d", invokeTime, 1)
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
	stopBurnCpuFunc = func() (bool, string) { return true, "" }

	channel = &cl.MockLocalChannel{
		Response:         spec.ReturnFail(spec.Code[spec.CommandNotFound], "taskset command not found"),
		ExpectedCommands: []string{fmt.Sprintf(`taskset -a -cp 0 25233`)},
		T:                t,
	}

	bindBurnCpuByTaskset(context.Background(), as.core, as.pid)
	if exitCode != 1 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}
}
func Test_checkBurnCpu(t *testing.T) {
	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	checkBurnCpu(context.Background())
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopBurnCpu()
		})
	}
}
