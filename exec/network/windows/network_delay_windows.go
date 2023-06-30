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
	"strconv"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/lib/windivert"
	"github.com/chaosblade-io/chaosblade-exec-os/lib/windivert/protocol"
)

var WidivertnetworkBin = "chaos_windivertnetwork.exe"

type WinNetDelayActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewWinNetDelayActionSpec() spec.ExpActionCommandSpec {
	return &WinNetDelayActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: commFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "time",
					Desc:     "Delay time, ms",
					Required: true,
				},
			},
			ActionProcessHang: true,
			ActionExecutor:    &WinNetworkDelayExecutor{},
		},
	}
}

func (*WinNetDelayActionSpec) Name() string {
	return "delay"
}

func (*WinNetDelayActionSpec) Aliases() []string {
	return []string{}
}

func (*WinNetDelayActionSpec) ShortDesc() string {
	return "Delay experiment"
}

func (*WinNetDelayActionSpec) LongDesc() string {
	return "Delay experiment"
}

type WinNetworkDelayExecutor struct {
	channel spec.Channel
}

func (de *WinNetworkDelayExecutor) Name() string {
	return "delay"
}

func (de *WinNetworkDelayExecutor) SetChannel(channel spec.Channel) {
	de.channel = channel
}

func (de *WinNetworkDelayExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	if de.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return de.stop(ctx)
	}

	time := model.ActionFlags["time"]
	if time == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "time")
	}
	delayTime, err := strconv.ParseInt(time, 10, 64)
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
	return de.start(uint64(delayTime), direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp, ctx)
}

func (de *WinNetworkDelayExecutor) start(delayOfTime uint64, direction, dstPort, srcPort, dstIp, srcIp, excludeDstPort, excludeSrcPort, excludeDstIp string, ctx context.Context) *spec.Response {
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
		time.Sleep(time.Duration(delayOfTime) * time.Millisecond)
		adapter.Send(data, addr)
	}
	return spec.Success()
}

func (de *WinNetworkDelayExecutor) stop(ctx context.Context) *spec.Response {

	ctx = context.WithValue(ctx, "bin", WidivertnetworkBin)
	return exec.Destroy(ctx, de.channel, "network delay")
}
