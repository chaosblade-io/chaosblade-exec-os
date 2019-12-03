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
	"testing"

	channel2 "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_startFill_startSuccessful(t *testing.T) {
	type args struct {
		path    string
		size    string
		percent string
	}
	as := &args{
		path:    "/dev",
		size:    "10",
		percent: "",
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	channel = &channel2.MockLocalChannel{
		Response: spec.ReturnSuccess("success"),
		NoCheck:  true,
		T:        t,
	}

	startFill(as.path, as.size, as.percent)
	if exitCode != 0 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}
}

func Test_stopFill(t *testing.T) {
	channel = &channel2.MockLocalChannel{
		Response: spec.ReturnSuccess("success"),
		NoCheck:  true,
		T:        t,
	}
	bin.ExitFunc = func(code int) {}
	type args struct {
		mountPoint string
	}
	tests := []struct {
		name string
		args args
	}{
		{"stop", args{"/dev"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopFill(tt.args.mountPoint)
		})
	}
}
