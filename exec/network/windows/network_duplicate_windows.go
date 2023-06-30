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
			ActionProcessHang: true,
			ActionExecutor:    &NetworkDuplicateExecutor{},
			ActionExample: `
# Specify the network card eth0 and repeat the packet by 10%
blade create network duplicate --percent=10 `,
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

	if de.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return de.stop(ctx)
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
	return de.start(uint64(percentInt), direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp, ctx)
}

func (de *NetworkDuplicateExecutor) start(percent uint64, direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp string, ctx context.Context) *spec.Response {
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
			adapter.Send(data, addr)
		}
		adapter.Send(data, addr)
	}
	return spec.Success()
}

func (de *NetworkDuplicateExecutor) stop(ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", WidivertnetworkBin)
	return exec.Destroy(ctx, de.channel, "network duplicate")
}

func (de *NetworkDuplicateExecutor) SetChannel(channel spec.Channel) {
	de.channel = channel
}
