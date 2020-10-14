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
	"flag"
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var dropSourcePort, dropDestinationPort, dropStringPattern, dropNetworkTraffic string
var dropNetStart, dropNetStop bool

func main() {
	flag.StringVar(&dropSourcePort, "source-port", "", "source port")
	flag.StringVar(&dropDestinationPort, "destination-port", "", "destination port")
	flag.StringVar(&dropStringPattern, "string-pattern", "", "string pattern")
	flag.StringVar(&dropNetworkTraffic, "network-traffic", "", "network traffic")
	flag.BoolVar(&dropNetStart, "start", false, "start drop")
	flag.BoolVar(&dropNetStop, "stop", false, "stop drop")
	bin.ParseFlagAndInitLog()

	if dropNetStart == dropNetStop {
		bin.PrintErrAndExit("must add --start or --stop flag")
	}
	if dropNetStart {
		startDropNet(dropSourcePort, dropDestinationPort, dropStringPattern, dropNetworkTraffic)
	} else if dropNetStop {
		stopDropNet(dropSourcePort, dropDestinationPort, dropStringPattern, dropNetworkTraffic)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

var stopDropNetFunc = stopDropNet

func startDropNet(sourcePort, destinationPort, stringPattern, networkTraffic string) {
	ctx := context.Background()
	if destinationPort == "" && sourcePort == "" && stringPattern == "" {
		bin.PrintErrAndExit("must specify port or string flag")
		return
	}
	handleDropSpecifyPort(destinationPort, sourcePort, stringPattern, networkTraffic, ctx)
}

func handleDropSpecifyPort(destinationPort string, sourcePort string, stringPattern string, networkTraffic string, ctx context.Context) {
	var response *spec.Response
	netFlows := []string{"INPUT", "OUTPUT"}
	if networkTraffic == "in" {
		netFlows = []string{"INPUT"}
	}
	if networkTraffic == "out" {
		netFlows = []string{"OUTPUT"}
	}
	for _, netFlow := range netFlows {
		tcpArgs := fmt.Sprintf("-A %s -p tcp", netFlow)
		udpArgs := fmt.Sprintf("-A %s -p udp", netFlow)
		if sourcePort != "" {
			tcpArgs = fmt.Sprintf("%s --sport %s", tcpArgs, sourcePort)
			udpArgs = fmt.Sprintf("%s --sport %s", udpArgs, sourcePort)
		}
		if destinationPort != "" {
			tcpArgs = fmt.Sprintf("%s --dport %s", tcpArgs, destinationPort)
			udpArgs = fmt.Sprintf("%s --dport %s", udpArgs, destinationPort)
		}
		if stringPattern != "" {
			tcpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", tcpArgs, stringPattern)
			udpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", udpArgs, stringPattern)
		}
		tcpArgs = fmt.Sprintf("%s -j DROP", tcpArgs)
		udpArgs = fmt.Sprintf("%s -j DROP", udpArgs)
		response = cl.Run(ctx, "iptables", fmt.Sprintf(`%s`, tcpArgs))
		if !response.Success {
			stopDropNetFunc(sourcePort, destinationPort, stringPattern, networkTraffic)
			bin.PrintErrAndExit(response.Err)
			return
		}
		response = cl.Run(ctx, "iptables", fmt.Sprintf(`%s`, udpArgs))
		if !response.Success {
			stopDropNetFunc(sourcePort, destinationPort, stringPattern, networkTraffic)
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func stopDropNet(sourcePort, destinationPort, stringPattern, networkTraffic string) {
	ctx := context.Background()
	var response *spec.Response
	netFlows := []string{"INPUT", "OUTPUT"}
	if networkTraffic == "in" {
		netFlows = []string{"INPUT"}
	}
	if networkTraffic == "out" {
		netFlows = []string{"OUTPUT"}
	}
	for _, netFlow := range netFlows {
		tcpArgs := fmt.Sprintf("-D %s -p tcp", netFlow)
		udpArgs := fmt.Sprintf("-D %s -p udp", netFlow)
		if sourcePort != "" {
			tcpArgs = fmt.Sprintf("%s --sport %s", tcpArgs, sourcePort)
			udpArgs = fmt.Sprintf("%s --sport %s", udpArgs, sourcePort)
		}
		if destinationPort != "" {
			tcpArgs = fmt.Sprintf("%s --dport %s", tcpArgs, destinationPort)
			udpArgs = fmt.Sprintf("%s --dport %s", udpArgs, destinationPort)
		}
		if stringPattern != "" {
			tcpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", tcpArgs, stringPattern)
			udpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", udpArgs, stringPattern)
		}
		tcpArgs = fmt.Sprintf("%s -j DROP", tcpArgs)
		udpArgs = fmt.Sprintf("%s -j DROP", udpArgs)
		response = cl.Run(ctx, "iptables", fmt.Sprintf(`%s`, tcpArgs))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
		response = cl.Run(ctx, "iptables", fmt.Sprintf(`%s`, udpArgs))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
