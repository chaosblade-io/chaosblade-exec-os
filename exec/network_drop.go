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
)

type DropActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDropActionSpec() spec.ExpActionCommandSpec {
	return &DropActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "local-port",
					Desc: "Port for local service",
				},
				&spec.ExpFlag{
					Name: "remote-port",
					Desc: "Port for remote service",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &NetworkDropExecutor{},
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

func (*DropActionSpec) LongDesc() string {
	return "Drop network data"
}

type NetworkDropExecutor struct {
	channel spec.Channel
}

func (*NetworkDropExecutor) Name() string {
	return "drop"
}

var dropNetworkBin = "chaos_dropnetwork"

func (ne *NetworkDropExecutor) Exec(suid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkNetworkDropExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if ne.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	localPort := model.ActionFlags["local-port"]
	remotePort := model.ActionFlags["remote-port"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return ne.stop(localPort, remotePort, ctx)
	}

	return ne.start(localPort, remotePort, ctx)
}

func (ne *NetworkDropExecutor) start(localPort, remotePort string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--start --debug=%t", util.Debug)
	if localPort != "" {
		args = fmt.Sprintf("%s --local-port %s", args, localPort)
	}
	if remotePort != "" {
		args = fmt.Sprintf("%s --remote-port %s", args, remotePort)
	}
	return ne.channel.Run(ctx, path.Join(ne.channel.GetScriptPath(), dropNetworkBin), args)
}

func (ne *NetworkDropExecutor) stop(localPort, remotePort string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("--stop --debug=%t", util.Debug)
	if localPort != "" {
		args = fmt.Sprintf("%s --local-port %s", args, localPort)
	}
	if remotePort != "" {
		args = fmt.Sprintf("%s --remote-port %s", args, remotePort)
	}
	return ne.channel.Run(ctx, path.Join(ne.channel.GetScriptPath(), dropNetworkBin), args)
}

func (ne *NetworkDropExecutor) SetChannel(channel spec.Channel) {
	ne.channel = channel
}

func checkNetworkDropExpEnv() error {
	commands := []string{"iptables"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
