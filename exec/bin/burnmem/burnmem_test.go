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

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	channel2 "github.com/chaosblade-io/chaosblade-spec-go/channel"
)

func Test_startBurnMem(t *testing.T) {

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	runBurnMemFunc = func(context.Context, int) int {
		return 1
	}

	stopBurnMemFunc = func() (bool, string) {
		return true, ""
	}

	flPath := path.Join(util.GetProgramPath(), dirName)
	channel = &channel2.MockLocalChannel{
		Response:         spec.ReturnSuccess("success"),
		ExpectedCommands: []string{fmt.Sprintf("mount -t tmpfs tmpfs %s -o size=", flPath) + "100%"},
		T:                t,
	}

	startBurnMem()
	if exitCode != 0 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 0)
	}

}

func Test_runBurnMem_failed(t *testing.T) {
	type args struct {
		memPercent int
	}
	as := &args{
		memPercent: 50,
	}

	burnBin := path.Join(util.GetProgramPath(), "chaos_burnmem")
	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	channel = &channel2.MockLocalChannel{
		Response:         spec.ReturnFail(spec.Code[spec.CommandNotFound], "nohup command not found"),
		ExpectedCommands: []string{fmt.Sprintf(`nohup %s --nohup --mem-percent 50 > /dev/null 2>&1 &`, burnBin)},
		T:                t,
	}

	stopBurnMemFunc = func() (bool, string) {
		return true, ""
	}

	runBurnMem(context.Background(), as.memPercent)

	if exitCode != 1 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}

}

func Test_stopBurnMem(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"stop"},
	}
	channel = channel2.NewLocalChannel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopBurnMem()
		})
	}
}
