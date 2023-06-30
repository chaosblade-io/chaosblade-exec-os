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
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/lib/windivert"
	"github.com/chaosblade-io/chaosblade-exec-os/lib/windivert/protocol"
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
			ActionProcessHang: true,
			ActionExecutor:    &NetworkReorderExecutor{},
			ActionExample: `# Access the specified IP request packet disorder
blade c network reorder --correlation 80 --percent 50 --gap 2 --time 500 --interface eth0 --destination-ip 180.101.49.12`,
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
	if ce.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return ce.stop(ctx)
	}

	percent := model.ActionFlags["percent"]
	if percent == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "percent")
	}
	percentInt, err := strconv.ParseInt(percent, 10, 64)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "percent", percent, err)
	}
	time := model.ActionFlags["time"]
	if time == "" {
		time = "10"
	}
	timedelay, err := strconv.ParseInt(time, 10, 64)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "time", time, err)
	}

	direction := model.ActionFlags["direction"]
	dstPort := model.ActionFlags["dst-port"]
	srcPort := model.ActionFlags["src-port"]
	dstIp := model.ActionFlags["dst-ip"]
	srcIp := model.ActionFlags["src-ip"]
	excludeDstPort := model.ActionFlags["exclude-dst-port"]
	excludeSrcPort := model.ActionFlags["exclude-src-port"]
	excludeDstIp := model.ActionFlags["exclude-dst-ip"]
	return ce.start(uint64(percentInt), timedelay, direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp, ctx)

}

func (ce *NetworkReorderExecutor) start(percent uint64, timedelay int64, direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort,
	excludeSrcPort, excludeDstIp string, ctx context.Context) *spec.Response {
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
		if uint64(random) <= percent {
			time.Sleep(time.Duration(timedelay) * time.Millisecond)
		}
		adapter.Send(data, addr)
	}
	// todo
	return spec.Success()
}

func (ce *NetworkReorderExecutor) stop(ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", WidivertnetworkBin)
	return exec.Destroy(ctx, ce.channel, "network reorder")
}

func (ce *NetworkReorderExecutor) SetChannel(channel spec.Channel) {
	ce.channel = channel
}
