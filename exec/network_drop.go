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

package exec

import (
	"context"
	"fmt"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

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
					Name: "string-pattern",
					Desc: "The string that is contained in the packet",
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

# Block outgoing connection to the specific domain
blade create network drop --string-pattern baidu.com --network-traffic out

# Block outgoing connection to the specific domain on port 80
blade create network drop --destination-port 80 --string-pattern baidu.com --network-traffic out
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
	commands := []string{"iptables"}
	if response, ok := channel.NewLocalChannel().IsAllCommandsAvailable(commands); !ok {
		return response
	}
	if ne.channel == nil {
		util.Errorf(suid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	sourceIp := model.ActionFlags["source-ip"]
	destinationIp := model.ActionFlags["destination-ip"]
	sourcePort := model.ActionFlags["source-port"]
	destinationPort := model.ActionFlags["destination-port"]
	stringPattern := model.ActionFlags["string-pattern"]
	networkTraffic := model.ActionFlags["network-traffic"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return ne.stop(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic, ctx)
	}

	return ne.start(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic, ctx)
}

func (ne *NetworkDropExecutor) start(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--start --debug=%t", util.Debug)
	if sourceIp != "" {
		args = fmt.Sprintf("%s --source-ip %s", args, sourceIp)
	}
	if destinationIp != "" {
		args = fmt.Sprintf("%s --destination-ip %s", args, destinationIp)
	}
	if sourcePort != "" {
		args = fmt.Sprintf("%s --source-port %s", args, sourcePort)
	}
	if destinationPort != "" {
		args = fmt.Sprintf("%s --destination-port %s", args, destinationPort)
	}
	if stringPattern != "" {
		args = fmt.Sprintf("%s --string-pattern %s", args, stringPattern)
	}
	if networkTraffic != "" {
		args = fmt.Sprintf("%s --network-traffic %s", args, networkTraffic)
	}
	return ne.channel.Run(ctx, path.Join(ne.channel.GetScriptPath(), DropNetworkBin), args)
}

func (ne *NetworkDropExecutor) stop(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--stop --debug=%t", util.Debug)
	if sourceIp != "" {
		args = fmt.Sprintf("%s --source-ip %s", args, sourceIp)
	}
	if destinationIp != "" {
		args = fmt.Sprintf("%s --destination-ip %s", args, destinationIp)
	}
	if sourcePort != "" {
		args = fmt.Sprintf("%s --source-port %s", args, sourcePort)
	}
	if destinationPort != "" {
		args = fmt.Sprintf("%s --destination-port %s", args, destinationPort)
	}
	if stringPattern != "" {
		args = fmt.Sprintf("%s --string-pattern %s", args, stringPattern)
	}
	if networkTraffic != "" {
		args = fmt.Sprintf("%s --network-traffic %s", args, networkTraffic)
	}
	return ne.channel.Run(ctx, path.Join(ne.channel.GetScriptPath(), DropNetworkBin), args)
}

func (ne *NetworkDropExecutor) SetChannel(channel spec.Channel) {
	ne.channel = channel
}
