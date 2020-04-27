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

func (*DuplicateActionSpec) LongDesc() string {
	return "Duplicate experiment"
}

type NetworkDuplicateExecutor struct {
	channel spec.Channel
}

func (de *NetworkDuplicateExecutor) Name() string {
	return "duplicate"
}

func (de *NetworkDuplicateExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkNetworkExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if de.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	netInterface := model.ActionFlags["interface"]
	if netInterface == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "less interface parameter")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return de.stop(netInterface, ctx)
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
		return de.start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent, ignorePeerPort, force, ctx)
	}
}

func (de *NetworkDuplicateExecutor) start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent string,
	ignorePeerPort, force bool, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--start --type duplicate --interface %s --percent %s --debug=%t", netInterface, percent, util.Debug)
	args, err := getCommArgs(localPort, remotePort, excludePort, destIp, excludeIp, args, ignorePeerPort, force)
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
	}
	return de.channel.Run(ctx, path.Join(de.channel.GetScriptPath(), tcNetworkBin), args)
}

func (de *NetworkDuplicateExecutor) stop(netInterface string, ctx context.Context) *spec.Response {
	return de.channel.Run(ctx, path.Join(de.channel.GetScriptPath(), tcNetworkBin),
		fmt.Sprintf("--stop --type duplicate --interface %s --debug=%t", netInterface, util.Debug))
}

func (de *NetworkDuplicateExecutor) SetChannel(channel spec.Channel) {
	de.channel = channel
}
