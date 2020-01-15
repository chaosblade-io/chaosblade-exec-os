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
	"fmt"
	"path"
	"testing"

	channel2 "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_startBurnIO_startFailed(t *testing.T) {
	type args struct {
		directory string
		size      string
		read      bool
		write     bool
	}

	burnBin := path.Join(util.GetProgramPath(), "chaos_burnio")
	as := &args{
		directory: "/home/admin",
		size:      "1024",
		read:      true,
		write:     true,
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	var invokeTime int
	stopBurnIOFunc = func(directory string, read, write bool) {
		invokeTime++
	}
	channel = &channel2.MockLocalChannel{
		Response:         spec.ReturnFail(spec.Code[spec.CommandNotFound], "nohup command not found"),
		ExpectedCommands: []string{fmt.Sprintf(`nohup %s --directory /home/admin --size 1024 --read=true --write=true --nohup=true > %s 2>&1 &`, burnBin, logFile)},
		T:                t,
	}

	startBurnIO(as.directory, as.size, as.read, as.write)
	if exitCode != 1 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 1)
	}
	if invokeTime != 1 {
		t.Errorf("unexpected invoke time %d, expected result: %d", invokeTime, 1)
	}
}

func Test_stopBurnIO(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		read      bool
		write     bool
	}{
		{
			name:      "stop",
			directory: "/home/admin",
			read:      true,
			write:     true,
		},
	}
	channel = channel2.NewLocalChannel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopBurnIO(tt.directory, tt.read, tt.write)
		})
	}
}
