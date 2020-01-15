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
	"testing"

	channel2 "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

func Test_createDnsPair(t *testing.T) {
	type input struct {
		domain string
		ip     string
	}
	tests := []struct {
		input  input
		expect string
	}{
		{input{"bbc.com", "151.101.8.81"}, "151.101.8.81 bbc.com #chaosblade"},
		{input{"g.com", "172.217.168.209"}, "172.217.168.209 g.com #chaosblade"},
		{input{"github.com", "192.30.255.112"}, "192.30.255.112 github.com #chaosblade"},
	}

	for _, tt := range tests {
		got := createDnsPair(tt.input.domain, tt.input.ip)
		if got != tt.expect {
			t.Errorf("unexpected result: %s, expected result: %s", got, tt.expect)
		}
	}
}
func Test_startChangeDns_failed(t *testing.T) {
	type args struct {
		domain string
		ip     string
	}

	as := &args{
		domain: "abc.com",
		ip:     "208.80.152.2",
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	channel = &channel2.MockLocalChannel{
		Response:         spec.ReturnSuccess("DnsPair has exists"),
		ExpectedCommands: []string{fmt.Sprintf(`grep -q "208.80.152.2 abc.com #chaosblade" /etc/hosts`)},
		T:                t,
	}

	startChangeDns(as.domain, as.ip)
	if exitCode != 1 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}
}

func Test_recoverDns_failed(t *testing.T) {
	type args struct {
		domain string
		ip     string
	}

	as := &args{
		domain: "abc.com",
		ip:     "208.80.152.2",
	}

	var exitCode int
	bin.ExitFunc = func(code int) {
		exitCode = code
	}
	channel = &channel2.MockLocalChannel{
		Response:         spec.ReturnFail(spec.Code[spec.CommandNotFound], "grep command not found"),
		ExpectedCommands: []string{fmt.Sprintf(`grep -q "208.80.152.2 abc.com #chaosblade" /etc/hosts`)},
		T:                t,
	}

	recoverDns(as.domain, as.ip)
	if exitCode != 0 {
		t.Errorf("unexpected result %d, expected result: %d", exitCode, 1)
	}
}
