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
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type ReorderActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewReorderActionSpec() spec.ExpActionCommandSpec {
	return &ReorderActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: commFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "percent",
					Desc:     "Packets are sent immediately percentage, must be positive integer without %, for example, --percent 50",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "correlation",
					Desc:     "Correlation on previous packet, value is between 0 and 100",
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "gap",
					Desc:     "Packet gap, must be positive integer",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "time",
					Desc:     "Delay time, must be positive integer, unit is millisecond, default value is 10",
					Required: false,
				},
			},
			ActionExecutor: &NetworkReorderExecutor{},
			ActionExample: `# Access the specified IP request packet disorder
blade c network reorder --correlation 80 --percent 50 --gap 2 --time 500 --interface eth0 --destination-ip 180.101.49.12`,
			ActionPrograms:   []string{TcNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*ReorderActionSpec) Name() string {
	return "reorder"
}

func (*ReorderActionSpec) Aliases() []string {
	return []string{}
}

func (*ReorderActionSpec) ShortDesc() string {
	return "Reorder experiment"
}

func (r *ReorderActionSpec) LongDesc() string {
	if r.ActionLongDesc != "" {
		return r.ActionLongDesc
	}
	return "Reorder experiment"
}

type NetworkReorderExecutor struct {
	channel spec.Channel
}

func (ce *NetworkReorderExecutor) Name() string {
	return "recorder"
}

func (ce *NetworkReorderExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"tc", "head"}
	if response, ok := ce.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	netInterface := model.ActionFlags["interface"]
	if netInterface == "" {
		log.Errorf(ctx, "interface is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "interface")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(netInterface, ctx)
	} else {
		percent := model.ActionFlags["percent"]
		if percent == "" {
			log.Errorf(ctx, "percent i nil")
			return spec.ResponseFailWithFlags(spec.ParameterLess, "percent")
		}
		gap := model.ActionFlags["gap"]
		time := model.ActionFlags["time"]
		if time == "" {
			time = "10"
		}
		correlation := model.ActionFlags["correlation"]
		if correlation == "" {
			correlation = "0"
		}
		localPort := model.ActionFlags["local-port"]
		remotePort := model.ActionFlags["remote-port"]
		excludePort := model.ActionFlags["exclude-port"]
		destIp := model.ActionFlags["destination-ip"]
		excludeIp := model.ActionFlags["exclude-ip"]
		ignorePeerPort := model.ActionFlags["ignore-peer-port"] == "true"
		protocol := model.ActionFlags["protocol"]
		force := model.ActionFlags["force"] == "true"
		return ce.start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent,
			ignorePeerPort, gap, time, correlation, force, protocol, ctx)
	}
}

func (ce *NetworkReorderExecutor) start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent string,
	ignorePeerPort bool, gap, time, correlation string, force bool, protocol string, ctx context.Context) *spec.Response {

	classRule := fmt.Sprintf("netem reorder %s%% %s%%", percent, correlation)
	if gap != "" {
		classRule = fmt.Sprintf("%s gap %s", classRule, gap)
	}
	classRule = fmt.Sprintf("%s delay %sms", classRule, time)

	return startNet(ctx, netInterface, classRule, localPort, remotePort, excludePort, destIp, excludeIp, force, ignorePeerPort, protocol, ce.channel)

}

func (ce *NetworkReorderExecutor) stop(netInterface string, ctx context.Context) *spec.Response {
	return stopNet(ctx, netInterface, ce.channel)
}

func (ce *NetworkReorderExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}
