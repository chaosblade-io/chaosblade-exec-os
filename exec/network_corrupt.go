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

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type CorruptActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewCorruptActionSpec() spec.ExpActionCommandSpec {
	return &CorruptActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: commFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "percent",
					Desc:     "Corruption percent, must be positive integer without %, for example, --percent 50",
					Required: true,
				},
			},
			ActionExecutor: &NetworkCorruptExecutor{},
		},
	}
}

func (*CorruptActionSpec) Name() string {
	return "corrupt"
}

func (*CorruptActionSpec) Aliases() []string {
	return []string{}
}

func (*CorruptActionSpec) ShortDesc() string {
	return "Corrupt experiment"
}

func (*CorruptActionSpec) LongDesc() string {
	return "Corrupt experiment"
}

type NetworkCorruptExecutor struct {
	channel spec.Channel
}

func (ce *NetworkCorruptExecutor) Name() string {
	return "corrupt"
}

func (ce *NetworkCorruptExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkNetworkExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if ce.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	netInterface := model.ActionFlags["interface"]
	if netInterface == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "less interface parameter")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(netInterface, ctx)
	} else {
		percent := model.ActionFlags["percent"]
		if percent == "" {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters], "less percent flag")
		}
		localPort := model.ActionFlags["local-port"]
		remotePort := model.ActionFlags["remote-port"]
		excludePort := model.ActionFlags["exclude-port"]
		destIp := model.ActionFlags["destination-ip"]
		excludeIp := model.ActionFlags["exclude-ip"]
		ignorePeerPort := model.ActionFlags["ignore-peer-port"] == "true"
		force := model.ActionFlags["force"] == "true"
		return ce.start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent, ignorePeerPort, force, ctx)
	}
}

func (ce *NetworkCorruptExecutor) start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent string,
	ignorePeerPort, force bool, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--start --type corrupt --interface %s --percent %s --debug=%t", netInterface, percent, util.Debug)
	args, err := getCommArgs(localPort, remotePort, excludePort, destIp, excludeIp, args, ignorePeerPort, force)
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
	}
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), tcNetworkBin), args)
}

func (ce *NetworkCorruptExecutor) stop(netInterface string, ctx context.Context) *spec.Response {
	return ce.channel.Run(ctx, path.Join(ce.channel.GetScriptPath(), tcNetworkBin),
		fmt.Sprintf("--stop --type corrupt --interface %s --debug=%t", netInterface, util.Debug))
}

func (ce *NetworkCorruptExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}
