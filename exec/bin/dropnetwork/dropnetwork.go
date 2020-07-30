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

var dropLocalPort, dropRemotePort string
var dropNetStart, dropNetStop bool

func main() {
	flag.StringVar(&dropLocalPort, "local-port", "", "local port")
	flag.StringVar(&dropRemotePort, "remote-port", "", "remote port")
	flag.BoolVar(&dropNetStart, "start", false, "start drop")
	flag.BoolVar(&dropNetStop, "stop", false, "stop drop")
	bin.ParseFlagAndInitLog()

	if dropNetStart == dropNetStop {
		bin.PrintErrAndExit("must add --start or --stop flag")
	}
	if dropNetStart {
		startDropNet(dropLocalPort, dropRemotePort)
	} else if dropNetStop {
		stopDropNet(dropLocalPort, dropRemotePort)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

var stopDropNetFunc = stopDropNet

func startDropNet(localPort, remotePort string) {
	ctx := context.Background()
	if remotePort == "" && localPort == "" {
		bin.PrintErrAndExit("must specify port flag")
		return
	}
	handleDropSpecifyPort(remotePort, localPort, ctx)
}

func handleDropSpecifyPort(remotePort string, localPort string, ctx context.Context) {
	var response *spec.Response
	if localPort != "" {
		response = cl.Run(ctx, "iptables",
			fmt.Sprintf(`-A INPUT -p tcp --dport %s -j DROP`, localPort))
		if !response.Success {
			stopDropNetFunc(localPort, remotePort)
			bin.PrintErrAndExit(response.Err)
			return
		}
		response = cl.Run(ctx, "iptables",
			fmt.Sprintf(`-A INPUT -p udp --dport %s -j DROP`, localPort))
		if !response.Success {
			stopDropNetFunc(localPort, remotePort)
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	if remotePort != "" {
		response = cl.Run(ctx, "iptables",
			fmt.Sprintf(`-A OUTPUT -p tcp --dport %s -j DROP`, remotePort))
		if !response.Success {
			stopDropNetFunc(localPort, remotePort)
			bin.PrintErrAndExit(response.Err)
			return
		}
		response = cl.Run(ctx, "iptables",
			fmt.Sprintf(`-A OUTPUT -p udp --dport %s -j DROP`, remotePort))
		if !response.Success {
			stopDropNetFunc(localPort, remotePort)
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func stopDropNet(localPort, remotePort string) {
	ctx := context.Background()
	if localPort != "" {
		cl.Run(ctx, "iptables", fmt.Sprintf(`-D INPUT -p tcp --dport %s -j DROP`, localPort))
		cl.Run(ctx, "iptables", fmt.Sprintf(`-D INPUT -p udp --dport %s -j DROP`, localPort))
	}
	if remotePort != "" {
		cl.Run(ctx, "iptables", fmt.Sprintf(`-D OUTPUT -p tcp --dport %s -j DROP`, remotePort))
		cl.Run(ctx, "iptables", fmt.Sprintf(`-D OUTPUT -p udp --dport %s -j DROP`, remotePort))
	}
}
