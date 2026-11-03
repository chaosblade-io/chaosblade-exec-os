/*
 * Copyright 2025 The ChaosBlade Authors
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

package tc

import (
	"math/rand"
	"reflect"
	"testing"
)

type buildtargetfilterparam = struct {
	input struct {
		localPortRanges   [][]int
		remotePortRanges  [][]int
		destIpRules       []string
		excludePortRanges [][]int
		excludeIpRules    []string
		args              string
		netInterface      string
		protocol          string
	}
	expect string
}

func TestBuildTargetFilterPortAndIp(t *testing.T) {
	var tests []buildtargetfilterparam
	var test1, test2 buildtargetfilterparam
	test1.input.remotePortRanges = append(test1.input.remotePortRanges, []int{6000, 9000})
	test1.input.destIpRules = append(test1.input.destIpRules, "match ip dst 10.18.2.156")
	test1.input.args = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms"
	test1.input.netInterface = "ens33"
	test1.input.protocol = "6"
	test1.expect = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 6000 0xfff0  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 6016 0xff80  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 6144 0xf800  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 8192 0xfe00  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 8704 0xff00  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 8960 0xffe0  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 8992 0xfff8  \\\n                                         match ip protocol 6 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.156 match ip dport 9000 0xffff  \\\n                                         match ip protocol 6 0xff flowid 1:4"

	test2.input.localPortRanges = append(test2.input.localPortRanges, []int{6000, 6001})
	test2.input.remotePortRanges = append(test2.input.remotePortRanges, []int{7000, 7010})
	test2.input.destIpRules = append(test2.input.destIpRules, "match ip dst 10.18.2.50")
	test2.input.excludePortRanges = append(test2.input.excludePortRanges, []int{7005, 7010})
	test2.input.excludeIpRules = append(test2.input.excludeIpRules, "match ip dst 10.18.1.138")
	test2.input.protocol = "17"
	test2.input.args = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms"
	test2.input.netInterface = "ens33"
	test2.expect = "qdisc add dev ens33 parent 1:4 handle 40: netem delay 200ms 0ms && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.50 match ip sport 6000 0xfffe  \\\n                                         match ip protocol 17 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.50 match ip dport 7000 0xfff8  \\\n                                         match ip protocol 17 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.50 match ip dport 7008 0xfffe  \\\n                                         match ip protocol 17 0xff flowid 1:4 && \\\n                            tc filter add dev ens33 parent 1: prio 4 protocol ip u32 match ip dst 10.18.2.50 match ip dport 7010 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:4 && \\\n\t\t\t\ttc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip dst 10.18.1.138  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip dport 7005 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip sport 7005 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip dport 7006 0xfffe  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip sport 7006 0xfffe  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip dport 7008 0xfffe  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip sport 7008 0xfffe  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip dport 7010 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:3 && \\\n                    tc filter add dev ens33 parent 1: prio 1 protocol ip u32 match ip sport 7010 0xffff  \\\n                                         match ip protocol 17 0xff flowid 1:3"
	tests = append(tests, test1, test2)

	for _, tt := range tests {
		// localPort, remotePort string, destIpRules, excludePorts, excludeIpRules []string, args string, netInterface, protocol string
		returnargs := buildTargetFilterPortAndIp(tt.input.localPortRanges, tt.input.remotePortRanges, tt.input.destIpRules, tt.input.excludePortRanges, tt.input.excludeIpRules, tt.input.args, tt.input.netInterface, tt.input.protocol)
		if returnargs != tt.expect {
			t.Errorf("unexpected result: %s, expected: %s", returnargs, tt.expect)
		}
	}
}

func TestBuildMaskForRange(t *testing.T) {
	start := rand.Int31n(65535)
	end := rand.Int31n(65535)
	if start > end {
		temp := start
		start = end
		end = temp
	}
	masks := buildMaskForRange(int(start), int(end))
	for i := 1; i <= 65535; i++ {
		if i < int(start) || i > int(end) {
			if isMatch(masks, uint16(i)) {
				t.Errorf("unexpected result: %d matched mask for [%d, %d]", i, start, end)
			}
		} else {
			if !isMatch(masks, uint16(i)) {
				t.Errorf("unexpected result: %d not matched mask for [%d, %d]", i, start, end)
			}
		}
	}
}

func Test_portSetToPortRanges(t *testing.T) {
	type args struct {
		portSet map[int]interface{}
	}
	tests := []struct {
		name string
		args args
		want [][]int
	}{
		{
			name: "one",
			args: args{
				portSet: map[int]interface{}{
					22: struct{}{},
				},
			},
			want: [][]int{{22, 22}},
		},
		{
			name: "two",
			args: args{
				portSet: map[int]interface{}{
					22:    struct{}{},
					19527: struct{}{},
				},
			},
			want: [][]int{{22, 22}, {19527, 19527}},
		},
		{
			name: "range",
			args: args{
				portSet: map[int]interface{}{
					22:   struct{}{},
					8000: struct{}{},
					8001: struct{}{},
					8002: struct{}{},
				},
			},
			want: [][]int{{22, 22}, {8000, 8002}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portSetToPortRanges(tt.args.portSet); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("portSetToPortRanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func isMatch(masks [][]uint16, target uint16) bool {
	for _, mask := range masks {
		v := mask[0]
		m := mask[1]
		if (target & m) == v {
			return true
		}
	}
	return false
}

func TestParseIntegerListToPortRanges(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [][]int
		wantErr bool
	}{
		{name: "single", input: "80", want: [][]int{{80, 80}}},
		{name: "list and range", input: "80,8000-8080", want: [][]int{{80, 80}, {8000, 8080}}},
		{name: "max single port", input: "65535", want: [][]int{{65535, 65535}}},
		{name: "range ending at max port", input: "60000-65535", want: [][]int{{60000, 65535}}},
		{name: "zero port", input: "0", wantErr: true},
		{name: "single port too large", input: "70000", wantErr: true},
		{name: "range end too large", input: "60000-65536", wantErr: true},
		{name: "range start zero", input: "0-10", wantErr: true},
		{name: "reversed range", input: "90-80", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIntegerListToPortRanges("local-port", tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseIntegerListToPortRanges(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIntegerListToPortRanges(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
