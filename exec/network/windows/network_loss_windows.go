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
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/lib/windivert"
	"github.com/chaosblade-io/chaosblade-exec-os/lib/windivert/protocol"
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
			ActionProcessHang: true,
			ActionExecutor:    &NetworkLossExecutor{},
			ActionExample: `
# Access to native 8080 and 8081 ports lost 70% of packets
blade create network loss --percent 70 --local-port 8080,8081

# The machine accesses external 14.215.177.39 machine (ping www.baidu.com) 80 port packet loss rate 100%
blade create network loss --percent 100 --remote-port 80 --destination-ip 14.215.177.39

# Do 60% packet loss for the entire network card Eth0, excluding ports 22 and 8000 to 8080
blade create network loss --percent 60 --exclude-port 22,8000-8080

# Realize the whole network card is not accessible, not accessible time 20 seconds. After executing the following command, the current network is disconnected and restored in 20 seconds. Remember!! Don't forget -timeout parameter
blade create network loss --percent 100 --interface eth0 --timeout 20`,
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

	if nle.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return nle.stop(ctx)
	}

	percent := model.ActionFlags["percent"]
	if percent == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "percent")
	}
	percentInt, err := strconv.ParseInt(percent, 10, 64)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "percent", percent, err)
	}
	direction := model.ActionFlags["direction"]
	dstPort := model.ActionFlags["dst-port"]
	srcPort := model.ActionFlags["src-port"]
	dstIp := model.ActionFlags["dst-ip"]
	srcIp := model.ActionFlags["src-ip"]
	excludeDstPort := model.ActionFlags["exclude-dst-port"]
	excludeSrcPort := model.ActionFlags["exclude-src-port"]
	excludeDstIp := model.ActionFlags["exclude-dst-ip"]
	return nle.start(uint64(percentInt), direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp, ctx)
}

func (nle *NetworkLossExecutor) start(percent uint64, direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp string, ctx context.Context) *spec.Response {
	args := fmt.Sprintf("%s", direction)
	if dstPort != "" {
		args = fmt.Sprintf("%s and tcp.DstPort=%s", args, dstPort)
	}
	if srcPort != "" {
		args = fmt.Sprintf("%s and tcp.SrcPort=%s", args, srcPort)
	}
	if dstIp != "" {
		args = fmt.Sprintf("%s and ip.DstAddr=%s", args, dstIp)
	}
	if srcIp != "" {
		args = fmt.Sprintf("%s and ip.SrcAddr=%s", args, srcIp)
	}
	if excludeDstPort != "" {
		args = fmt.Sprintf("%s and tcp.DstPort!=%s", args, excludeDstPort)
	}
	if excludeSrcPort != "" {
		args = fmt.Sprintf("%s and tcp.SrcPort!=%s", args, excludeSrcPort)
	}
	if excludeDstIp != "" {
		args = fmt.Sprintf("%s and ip.DstAddr!=%s", args, excludeDstIp)
	}
	adapter, err := windivert.NewWindivertAdapter(args)
	if err != nil {
		log.Errorf(ctx, spec.OsCmdExecFailed.Sprintf("new windows divert adapter", err))
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "new windows divert adapter", err)
	}

	for true {
		data, addr := adapter.Recv()
		protocol := protocol.ParseProtocol(data)
		if protocol != nil {
			marshal, _ := json.Marshal(protocol)
			log.Infof(ctx, "protocol %s ", string(marshal))
		}
		random := rand.Int63n(100)
		if uint64(random) > percent {
			adapter.Send(data, addr)
		}
	}

	return spec.Success()
}
func (nle *NetworkLossExecutor) stop(ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", WidivertnetworkBin)
	return exec.Destroy(ctx, nle.channel, "network loss")
}

func (nle *NetworkLossExecutor) SetChannel(channel spec.Channel) {
	nle.channel = channel
}
