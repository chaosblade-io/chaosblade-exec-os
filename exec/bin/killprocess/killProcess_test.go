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
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_killProcess(t *testing.T) {
	type args struct {
		process     string
		processCmd  string
		localPorts  string
		count       int
		mockChannel *channel.MockLocalChannel
	}
	cl = channel.NewMockLocalChannel()
	mockChannel := cl.(*channel.MockLocalChannel)
	mockChannel.GetPidsByLocalPortsFunc = func(localPorts []string) (strings []string, e error) {
		return []string{"10000"}, nil
	}
	mockChannel.RunFunc = func(ctx context.Context, script, args string) *spec.Response {
		return spec.ReturnSuccess("success")
	}

	tests := []struct {
		name     string
		args     args
		exitCode int
		exitMsg  string
	}{
		{
			name:     "processNotFound",
			args:     args{process: "tomcat", mockChannel: mockChannel},
			exitCode: 1,
			exitMsg:  "tomcat process not found",
		},
		{
			name:     "processCmdNotFound",
			args:     args{processCmd: "java", mockChannel: mockChannel},
			exitCode: 1,
			exitMsg:  "java process not found",
		},
		{
			name:     "killProcessByLocalPortsSuccessfully",
			args:     args{localPorts: "8080", mockChannel: mockChannel},
			exitCode: 0,
			exitMsg:  "success",
		},
	}
	exitCode := 0
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			killProcess(tt.args.process, tt.args.processCmd, tt.args.localPorts, tt.args.count)
			if tt.exitCode != exitCode {
				t.Errorf("unexpected exitCode %d, expected exitCode: %d", exitCode, tt.exitCode)
			}
			if tt.exitMsg != bin.ExitMessageForTesting {
				t.Errorf("unexpected exitMsg %s, expected exitMsg: %s", bin.ExitMessageForTesting, tt.exitMsg)
			}
		})
	}
}
