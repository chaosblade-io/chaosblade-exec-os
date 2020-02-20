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

package main

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_startBurnMem(t *testing.T) {

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	runBurnMemFunc = func(context.Context, int, int, int, string) {
	}

	stopBurnMemFunc = func() (bool, string) {
		return true, ""
	}

	flPath := path.Join(util.GetProgramPath(), dirName)

	cl = channel.NewMockLocalChannel()
	mockChannel := cl.(*channel.MockLocalChannel)
	actualCommands := make([]string, 0)
	mockChannel.RunFunc = func(ctx context.Context, script, args string) *spec.Response {
		actualCommands = append(actualCommands, fmt.Sprintf("%s %s", script, args))
		return spec.ReturnSuccess("success")
	}
	expectedCommands := []string{fmt.Sprintf("mount -t tmpfs tmpfs %s -o size=", flPath) + "100%"}

	startBurnMem()
	if exitCode != 0 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 0)
	}
	if !reflect.DeepEqual(expectedCommands, actualCommands) {
		t.Errorf("unexpected commands: %+v, expected commands: %+v", actualCommands, expectedCommands)
	}
}

func Test_runBurnMem_failed(t *testing.T) {
	type args struct {
		memPercent  int
		memReserve  int
		memRate     int
		burnMemMode string
	}
	as := &args{
		memPercent: 50,
	}

	burnBin := path.Join(util.GetProgramPath(), "chaos_burnmem")
	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	cl = channel.NewMockLocalChannel()
	mockChannel := cl.(*channel.MockLocalChannel)
	actualCommands := make([]string, 0)
	mockChannel.RunFunc = func(ctx context.Context, script, args string) *spec.Response {
		actualCommands = append(actualCommands, fmt.Sprintf("%s %s", script, args))
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], "nohup command not found")
	}
	expectedCommands := []string{fmt.Sprintf(`nohup %s --nohup --mem-percent 50 > /dev/null 2>&1 &`, burnBin)}
	stopBurnMemFunc = func() (bool, string) {
		return true, ""
	}

	runBurnMem(context.Background(), as.memPercent, as.memReserve, as.memRate, as.burnMemMode)
	if exitCode != 1 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 1)
	}
	if !reflect.DeepEqual(expectedCommands, actualCommands) {
		t.Errorf("unexpected commands: %+v, expected commands: %+v", actualCommands, expectedCommands)
	}
}

func Test_stopBurnMem(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"stop"},
	}
	cl = channel.NewLocalChannel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopBurnMem()
		})
	}
}
