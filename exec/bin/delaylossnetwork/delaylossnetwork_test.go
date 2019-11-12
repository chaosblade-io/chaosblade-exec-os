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
	"testing"

	channel2 "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_startDelayNet(t *testing.T) {
	type args struct {
		netInterface string
		classRule    string
		localPort    string
		remotePort   string
		excludePort  string
		destIp       string
	}

	as := &args{
		netInterface: "eth0",
		classRule:    "netem delay 3000ms 10ms",
		localPort:    "",
		remotePort:   "",
		excludePort:  "",
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	channel = &channel2.MockLocalChannel{
		Response: spec.ReturnSuccess("success"),
		ExpectedCommands: []string{
			`ifconfig eth0 txqueuelen 1000`,
			fmt.Sprintf(`tc qdisc add dev eth0 root netem delay 3000ms 10ms`)},
		T: t,
	}
	startNet(as.netInterface, as.classRule, as.localPort, as.remotePort, as.excludePort, as.destIp)
	if exitCode != 0 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 1)
	}
}

func Test_addLocalOrRemotePortForDelay(t *testing.T) {
	type input struct {
		localPort        string
		remotePort       string
		netInterface     string
		response         *spec.Response
		expectedCommands []string
		classRule        string
		ipRule           string
	}
	type expect struct {
		exitCode   int
		invokeTime int
	}

	tests := []struct {
		input  input
		expect expect
	}{
		{input{"80", "", "eth0", spec.ReturnSuccess("success"),
			[]string{
				`tc qdisc add dev eth0 parent 1:4 handle 40: netem delay 3000ms 10ms`,
				`tc filter add dev eth0 parent 1: prio 4 protocol ip u32  match ip sport 80 0xffff flowid 1:4`,
			},
			"netem delay 3000ms 10ms", ""},
			expect{0, 0}},
		{input{"", "80", "eth0", spec.ReturnSuccess("success"),
			[]string{
				`tc qdisc add dev eth0 parent 1:4 handle 40: netem delay 3000ms 10ms`,
				`tc filter add dev eth0 parent 1: prio 4 protocol ip u32  match ip dport 80 0xffff flowid 1:4`,
			},
			"netem delay 3000ms 10ms", ""},
			expect{0, 0}},
		{input{"80", "", "eth0", spec.ReturnFail(spec.Code[spec.CommandNotFound], "tc command not found"),
			[]string{
				`tc qdisc add dev eth0 parent 1:4 handle 40: netem delay 3000ms 10ms`,
				`tc filter del dev eth0 parent 1: prio 4`,
				`tc qdisc del dev eth0 root`,
			},
			"netem delay 3000ms 10ms", ""},
			expect{1, 1}},
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}

	for _, tt := range tests {
		channel = &channel2.MockLocalChannel{
			Response:         tt.input.response,
			ExpectedCommands: tt.input.expectedCommands,
			T:                t,
		}
		// ctx context.Context, channel spec.Channel,
		//	netInterface, classRule, localPort, remotePort string
		addLocalOrRemotePortForDL(context.Background(), channel, tt.input.netInterface, tt.input.classRule, tt.input.localPort, tt.input.remotePort, tt.input.ipRule)
		if exitCode != tt.expect.exitCode {
			t.Errorf("unexpected result: %d, expected result: %d", exitCode, tt.expect.exitCode)
		}
	}
}

func Test_addExcludePortFilterForDelay(t *testing.T) {
	type input struct {
		excludePort      string
		netInterface     string
		response         *spec.Response
		expectedCommands []string
		classRule        string
		ipRule           string
	}
	type expect struct {
		exitCode   int
		invokeTime int
	}

	tests := []struct {
		input  input
		expect expect
	}{
		{input{"80", "eth0", spec.ReturnFail(spec.Code[spec.CommandNotFound], "tc command not found"),
			[]string{`tc qdisc add dev eth0 parent 1:1 netem delay 3000ms 10ms && \
			tc qdisc add dev eth0 parent 1:2 netem delay 3000ms 10ms && \
			tc qdisc add dev eth0 parent 1:3 netem delay 3000ms 10ms && \
			tc qdisc add dev eth0 parent 1:4 handle 40: pfifo_fast && \
			tc filter add dev eth0 parent 1: prio 4 protocol ip u32  match ip sport 80 0xffff flowid 1:4 && \
			tc filter add dev eth0 parent 1: prio 4 protocol ip u32  match ip dport 80 0xffff flowid 1:4`},
			"netem delay 3000ms 10ms", ""},
			expect{1, 1}},
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	var invokeTime int
	stopDLNetFunc = func(netInterface string) {
		invokeTime++
	}
	for _, tt := range tests {
		invokeTime = 0
		channel = &channel2.MockLocalChannel{
			Response:         tt.input.response,
			ExpectedCommands: tt.input.expectedCommands,
			T:                t,
		}
		addExcludePortFilterForDL(context.Background(), channel, tt.input.netInterface, tt.input.classRule, tt.input.excludePort, tt.input.ipRule)
		if exitCode != tt.expect.exitCode {
			t.Errorf("unexpected result: %d, expected result: %d", exitCode, tt.expect.exitCode)
		}
		if invokeTime != tt.expect.invokeTime {
			t.Errorf("unexpected invoke time %d, expected result: %d", invokeTime, tt.expect.invokeTime)
		}
	}
}

func Test_addQdiscForDelay(t *testing.T) {
	type args struct {
		netInterface string
		time         string
		offset       string
	}
	as := &args{
		netInterface: "eth0",
		time:         "3000",
		offset:       "10",
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	channel = &channel2.MockLocalChannel{
		Response:         spec.ReturnFail(spec.Code[spec.CommandNotFound], "tc command not found"),
		ExpectedCommands: []string{fmt.Sprintf(`tc qdisc add dev eth0 root handle 1: prio bands 4`)},
		T:                t,
	}

	addQdiscForDL(channel, context.Background(), as.netInterface)
	if exitCode != 1 {
		t.Errorf("unexpected result: %d, expected result: %d", exitCode, 1)
	}
}
