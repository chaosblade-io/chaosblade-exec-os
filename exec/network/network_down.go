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

package network

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const DownNetworkBin = "chaos_downnetwork"

type DownActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDownActionSpec() spec.ExpActionCommandSpec {
	return &DownActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:                  "device",
					Desc:                  "device name,the network interface to impact",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:                  "duration",
					Desc:                  "duration time,The unit of time is second, which can be decimal",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
			},
			ActionExecutor: &NetworkDownExecutor{},
			ActionExample: `
# down(close) the network device interface
create network down --device lo --duration 0.003 `,
			ActionPrograms:   []string{DownNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*DownActionSpec) Name() string {
	return "down"
}

func (*DownActionSpec) Aliases() []string {
	return []string{}
}

func (*DownActionSpec) ShortDesc() string {
	return "device down experiment"
}

func (d *DownActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "device down experiment"
}

type NetworkDownExecutor struct {
	channel spec.Channel
}

func (*NetworkDownExecutor) Name() string {
	return "down"
}

var changeDownBin = "chaos_DeviceDown"

func (ns *NetworkDownExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"ifconfig", "echo"}
	if response, ok := ns.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	device := model.ActionFlags["device"]
	duration := model.ActionFlags["duration"]

	if device == "" {
		log.Errorf(ctx, "network device interface is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "network device interface")
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return ns.down(ctx, device)
	}
	return ns.start(ctx, device, duration)
}

func (ns *NetworkDownExecutor) start(ctx context.Context, device, duration string) *spec.Response {
	nICDownCommand := fmt.Sprintf("  %s down", device)
	response := ns.channel.Run(ctx, "ifconfig", nICDownCommand)
	if !response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s has been close error", device))
	}

	if duration != "" {
		nICUpCommand := fmt.Sprintf("sleep %s && sudo ifconfig %s up", duration, device)
		response = ns.channel.Run(ctx, "", nICUpCommand)
	}
	return response
}

func (ns *NetworkDownExecutor) down(ctx context.Context, device string) *spec.Response {
	nICUpCommand := fmt.Sprintf("  %s up", device)
	return ns.channel.Run(ctx, "ifconfig", nICUpCommand)
}

func (ns *NetworkDownExecutor) SetChannel(channel spec.Channel) {
	ns.channel = channel
}
