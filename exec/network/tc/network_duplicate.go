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

package tc

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

type DuplicateActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDuplicateActionSpec() spec.ExpActionCommandSpec {
	return &DuplicateActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: commFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "percent",
					Desc:     "Duplication percent, must be positive integer without %, for example, --percent 50",
					Required: true,
				},
			},
			ActionExecutor: &NetworkDuplicateExecutor{},
			ActionExample: `
# Specify the network card eth0 and repeat the packet by 10%
blade create network duplicate --percent=10 --interface=eth0`,
			ActionPrograms:   []string{TcNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*DuplicateActionSpec) Name() string {
	return "duplicate"
}

func (*DuplicateActionSpec) Aliases() []string {
	return []string{}
}

func (*DuplicateActionSpec) ShortDesc() string {
	return "Duplicate experiment"
}

func (d *DuplicateActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "Duplicate experiment"
}

type NetworkDuplicateExecutor struct {
	channel spec.Channel
}

func (de *NetworkDuplicateExecutor) Name() string {
	return "duplicate"
}

func (de *NetworkDuplicateExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"tc", "head"}
	if response, ok := de.channel.IsAllCommandsAvailable(ctx, commands); !ok {
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
		percent := model.ActionFlags["percent"]
		if percent == "" {
			util.Errorf(uid, util.GetRunFuncName(), spec.ParameterLess.Sprintf("percent"))
			return spec.ResponseFailWithFlags(spec.ParameterLess, "interface")
		}
		localPort := model.ActionFlags["local-port"]
		remotePort := model.ActionFlags["remote-port"]
		excludePort := model.ActionFlags["exclude-port"]
		destIp := model.ActionFlags["destination-ip"]
		excludeIp := model.ActionFlags["exclude-ip"]
		ignorePeerPort := model.ActionFlags["ignore-peer-port"] == "true"
		force := model.ActionFlags["force"] == "true"
		return de.start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent, ignorePeerPort, force, ctx)
	}
}

func (de *NetworkDuplicateExecutor) start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent string,
	ignorePeerPort, force bool, ctx context.Context) *spec.Response {

	classRule := fmt.Sprintf("netem duplicate %s%%", percent)

	return startNet(ctx, netInterface, classRule, localPort, remotePort, excludePort, destIp, excludeIp, force, ignorePeerPort, de.channel)

}

func (de *NetworkDuplicateExecutor) stop(netInterface string, ctx context.Context) *spec.Response {
	return stopNet(netInterface, de.channel)
}

func (de *NetworkDuplicateExecutor) SetChannel(channel spec.Channel) {
	de.channel = channel
}
