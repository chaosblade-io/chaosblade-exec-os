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

package windows

import (
	"context"
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const DropNetworkBin = "chaos_dropnetwork"

type DropActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDropActionSpec() spec.ExpActionCommandSpec {
	return &DropActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "source-ip",
					Desc: "The source ip address of packet",
				},
				&spec.ExpFlag{
					Name: "destination-ip",
					Desc: "The destination ip address of packet",
				},
				&spec.ExpFlag{
					Name: "source-port",
					Desc: "The source port of packet",
				},
				&spec.ExpFlag{
					Name: "destination-port",
					Desc: "The destination port of packet",
				},
				&spec.ExpFlag{
					Name: "network-traffic",
					Desc: "The direction of network traffic",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &NetworkDropExecutor{},
			ActionExample: `
# Block incoming connection from the source ip 10.10.10.10
blade create network drop --source-ip 10.10.10.10 --network-traffic in

# Block incoming connection to the destination ip 10.10.10.10
blade create network drop --destination-ip 10.10.10.10 --network-traffic in

# Block incoming connection from the port 80
blade create network drop --source-port 80 --network-traffic in

# Block incoming connection to the port 80 and 81
blade create network drop --destination-port 80,81 --network-traffic in

# Block outgoing connection to the port 80
blade create network drop --destination-port 80 --network-traffic out
`,
			ActionPrograms:   []string{DropNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*DropActionSpec) Name() string {
	return "drop"
}

func (*DropActionSpec) Aliases() []string {
	return []string{}
}

func (*DropActionSpec) ShortDesc() string {
	return "Drop experiment"
}

func (d *DropActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "Drop network data"
}

type NetworkDropExecutor struct {
	channel spec.Channel
}

func (*NetworkDropExecutor) Name() string {
	return "drop"
}

func (ne *NetworkDropExecutor) Exec(suid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	sourceIp := model.ActionFlags["source-ip"]
	destinationIp := model.ActionFlags["destination-ip"]
	sourcePort := model.ActionFlags["source-port"]
	destinationPort := model.ActionFlags["destination-port"]
	networkTraffic := model.ActionFlags["network-traffic"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return ne.stop(sourceIp, destinationIp, sourcePort, destinationPort, networkTraffic, ctx)
	}

	return ne.start(sourceIp, destinationIp, sourcePort, destinationPort, networkTraffic, ctx)
}

const NetshRuleName = "chaosblade-rule"

func (ne *NetworkDropExecutor) start(sourceIp, destinationIp, sourcePort, destinationPort, networkTraffic string, ctx context.Context) *spec.Response {
	if destinationIp == "" && sourceIp == "" && destinationPort == "" && sourcePort == "" {
		return spec.ReturnFail(spec.OsCmdExecFailed, "must specify ip or port or string flag")
	}

	var response *spec.Response
	netFlows := []string{"in", "out"}
	if networkTraffic == "in" {
		netFlows = []string{"in"}
	}
	if networkTraffic == "out" {
		netFlows = []string{"out"}
	}
	for _, netFlow := range netFlows {
		tcpArgs := fmt.Sprintf("advfirewall firewall add rule name=%s dir=%s protocol=tcp", NetshRuleName, netFlow)
		udpArgs := fmt.Sprintf("advfirewall firewall add rule name=%s dir=%s protocol=udp", NetshRuleName, netFlow)
		if sourceIp != "" {
			tcpArgs = fmt.Sprintf("%s localip=%s", tcpArgs, sourceIp)
			udpArgs = fmt.Sprintf("%s localip=%s", udpArgs, sourceIp)
		}
		if destinationIp != "" {
			tcpArgs = fmt.Sprintf("%s remoteip=%s", tcpArgs, destinationIp)
			udpArgs = fmt.Sprintf("%s remoteip=%s", udpArgs, destinationIp)
		}
		if sourcePort != "" {
			tcpArgs = fmt.Sprintf("%s localport=%s", tcpArgs, sourcePort)
			udpArgs = fmt.Sprintf("%s localport=%s", udpArgs, sourcePort)
		}
		if destinationPort != "" {
			tcpArgs = fmt.Sprintf("%s remoteport=%s", tcpArgs, destinationPort)
			udpArgs = fmt.Sprintf("%s remoteport=%s", udpArgs, destinationPort)
		}
		tcpArgs = fmt.Sprintf("%s action=block", tcpArgs)
		udpArgs = fmt.Sprintf("%s action=block", udpArgs)
		response = ne.channel.Run(ctx, "netsh", fmt.Sprintf(`%s`, tcpArgs))
		if !response.Success {
			ne.stop(sourceIp, destinationIp, sourcePort, destinationPort, networkTraffic, ctx)
			return response
		}
		response = ne.channel.Run(ctx, "netsh", fmt.Sprintf(`%s`, udpArgs))
		if !response.Success {
			ne.stop(sourceIp, destinationIp, sourcePort, destinationPort, networkTraffic, ctx)
		}
	}
	return response
}

func (ne *NetworkDropExecutor) stop(sourceIp, destinationIp, sourcePort, destinationPort, networkTraffic string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("advfirewall firewall delete rule name=%s", NetshRuleName)
	return ne.channel.Run(ctx, "netsh", fmt.Sprintf(`%s`, args))
}

func (ne *NetworkDropExecutor) SetChannel(channel spec.Channel) {
	ne.channel = channel
}
