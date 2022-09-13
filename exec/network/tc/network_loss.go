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

type LossActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewLossActionSpec() spec.ExpActionCommandSpec {
	return &LossActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: commFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "percent",
					Desc:     "loss percent, [0, 100]",
					Required: true,
				},
			},
			ActionExecutor: &NetworkLossExecutor{},
			ActionExample: `
# Access to native 8080 and 8081 ports lost 70% of packets
blade create network loss --percent 70 --interface eth0 --local-port 8080,8081

# The machine accesses external 14.215.177.39 machine (ping www.baidu.com) 80 port packet loss rate 100%
blade create network loss --percent 100 --interface eth0 --remote-port 80 --destination-ip 14.215.177.39

# Do 60% packet loss for the entire network card Eth0, excluding ports 22 and 8000 to 8080
blade create network loss --percent 60 --interface eth0 --exclude-port 22,8000-8080

# Realize the whole network card is not accessible, not accessible time 20 seconds. After executing the following command, the current network is disconnected and restored in 20 seconds. Remember!! Don't forget -timeout parameter
blade create network loss --percent 100 --interface eth0 --timeout 20`,
			ActionPrograms:   []string{TcNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*LossActionSpec) Name() string {
	return "loss"
}

func (*LossActionSpec) Aliases() []string {
	return []string{}
}

func (*LossActionSpec) ShortDesc() string {
	return "Loss network package"
}

func (l *LossActionSpec) LongDesc() string {
	if l.ActionLongDesc != "" {
		return l.ActionLongDesc
	}
	return "Loss network package"
}

type NetworkLossExecutor struct {
	channel spec.Channel
}

func (*NetworkLossExecutor) Name() string {
	return "loss"
}

func (nle *NetworkLossExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"tc", "head"}
	if response, ok := nle.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	var dev = ""
	if netInterface, ok := model.ActionFlags["interface"]; ok {
		if netInterface == "" {
			log.Errorf(ctx,"interface is nil")
			return spec.ResponseFailWithFlags(spec.ParameterLess, "interface")
		}
		dev = netInterface
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return nle.stop(dev, ctx)
	}
	percent := model.ActionFlags["percent"]
	if percent == "" {
		log.Errorf(ctx, "percent is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "percent")
	}
	localPort := model.ActionFlags["local-port"]
	remotePort := model.ActionFlags["remote-port"]
	excludePort := model.ActionFlags["exclude-port"]
	destIp := model.ActionFlags["destination-ip"]
	excludeIp := model.ActionFlags["exclude-ip"]
	ignorePeerPort := model.ActionFlags["ignore-peer-port"] == "true"
	protocol := model.ActionFlags["protocol"]
	force := model.ActionFlags["force"] == "true"
	return nle.start(dev, localPort, remotePort, excludePort, destIp, excludeIp, percent, ignorePeerPort, force, protocol, ctx)
}

func (nle *NetworkLossExecutor) start(netInterface, localPort, remotePort, excludePort, destIp, excludeIp, percent string,
	ignorePeerPort, force bool, protocol string, ctx context.Context) *spec.Response {
	classRule := fmt.Sprintf("netem loss %s%%", percent)
	return startNet(ctx, netInterface, classRule, localPort, remotePort, excludePort, destIp, excludeIp, force, ignorePeerPort, protocol, nle.channel)

}

func (nle *NetworkLossExecutor) stop(netInterface string, ctx context.Context) *spec.Response {
	return stopNet(ctx, netInterface, nle.channel)
}

func (nle *NetworkLossExecutor) SetChannel(channel spec.Channel) {
	nle.channel = channel
}
