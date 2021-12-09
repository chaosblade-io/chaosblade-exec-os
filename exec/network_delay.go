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

type DelayActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDelayActionSpec() spec.ExpActionCommandSpec {
	return &DelayActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: commFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "time",
					Desc:     "Delay time, ms",
					Required: true,
				},
				&spec.ExpFlag{
					Name: "offset",
					Desc: "Delay offset time, ms",
				},
			},
			ActionExecutor: &NetworkDelayExecutor{},
			ActionExample: `
# Access to native 8080 and 8081 ports is delayed by 3 seconds, and the delay time fluctuates by 1 second
blade create network delay --time 3000 --offset 1000 --interface eth0 --local-port 8080,8081

# Local access to external 14.215.177.39 machine (ping www.baidu.com obtained IP) port 80 delay of 3 seconds
blade create network delay --time 3000 --interface eth0 --remote-port 80 --destination-ip 14.215.177.39

# Do a 5 second delay for the entire network card eth0, excluding ports 22 and 8000 to 8080
blade create network delay --time 5000 --interface eth0 --exclude-port 22,8000-8080`,
			ActionPrograms:   []string{TcNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*DelayActionSpec) Name() string {
	return "delay"
}

func (*DelayActionSpec) Aliases() []string {
	return []string{}
}

func (*DelayActionSpec) ShortDesc() string {
	return "Delay experiment"
}

func (d *DelayActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "Delay experiment"
}

type NetworkDelayExecutor struct {
	channel spec.Channel
}

func (de *NetworkDelayExecutor) Name() string {
	return "delay"
}

func (de *NetworkDelayExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"tc", "head"}
	if response, ok := channel.NewLocalChannel().IsAllCommandsAvailable(commands); !ok {
		return response
	}

	if de.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	netInterface := model.ActionFlags["interface"]
	if netInterface == "" {
		util.Errorf(uid, util.GetRunFuncName(), spec.ParameterLess.Sprintf("interface"))
		return spec.ResponseFailWithFlags(spec.ParameterLess, "interface")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return de.stop(netInterface, ctx)
	} else {
		time := model.ActionFlags["time"]
		if time == "" {
			util.Errorf(uid, util.GetRunFuncName(), spec.ParameterLess.Sprintf("time"))
			return spec.ResponseFailWithFlags(spec.ParameterLess, "time")
		}
		offset := model.ActionFlags["offset"]
		if offset == "" {
			offset = "0"
		}
		localPort := model.ActionFlags["local-port"]
		remotePort := model.ActionFlags["remote-port"]
		excludePort := model.ActionFlags["exclude-port"]
		destIp := model.ActionFlags["destination-ip"]
		excludeIp := model.ActionFlags["exclude-ip"]
		ignorePeerPort := model.ActionFlags["ignore-peer-port"] == "true"
		force := model.ActionFlags["force"] == "true"
		return de.start(localPort, remotePort, excludePort, destIp, excludeIp, time, offset, netInterface, ignorePeerPort, force, ctx)
	}
}

func (de *NetworkDelayExecutor) start(localPort, remotePort, excludePort, destIp, excludeIp, time, offset, netInterface string,
	ignorePeerPort, force bool, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--start --type delay --interface %s --time %s --offset %s --debug=%t", netInterface, time, offset, util.Debug)
	args, response := getCommArgs(localPort, remotePort, excludePort, destIp, excludeIp, args, ignorePeerPort, force)
	if !response.Success {
		return response
	}
	return de.channel.Run(ctx, path.Join(de.channel.GetScriptPath(), TcNetworkBin), args)
}

func (de *NetworkDelayExecutor) stop(netInterface string, ctx context.Context) *spec.Response {
	return de.channel.Run(ctx, path.Join(de.channel.GetScriptPath(), TcNetworkBin),
		fmt.Sprintf("--stop --type delay --interface %s --debug=%t", netInterface, util.Debug))
}

func (de *NetworkDelayExecutor) SetChannel(channel spec.Channel) {
	de.channel = channel
}
