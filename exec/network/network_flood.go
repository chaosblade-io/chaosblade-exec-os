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
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
	"net"
	"strings"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const FloodNetworkBin = "chaos_floodnetwork"

type FloodActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFloodActionSpec() spec.ExpActionCommandSpec {
	return &FloodActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:                  "ip",
					Desc:                  "generate traffic to this IP address",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:                  "parallel",
					Desc:                  "number of iperf parallel client threads to run, default 1",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:                  "port",
					Desc:                  "generate traffic to this port on the IP address",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:                  "rate",
					Desc:                  "the speed of network traffic, allows bps, kbps, mbps, gbps, tbps unit. bps means bytes per second",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:                  "duration",
					Desc:                  "number of seconds to run the iperf test",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
			},
			ActionExecutor: &NetworkFloodExecutor{},
			ActionExample: `
# Flood(close) the network device interface
create network flood --ip=172.16.93.128 --duration=30  --port 5201 --rate=10m`,
			ActionPrograms:   []string{FloodNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*FloodActionSpec) Name() string {
	return "flood"
}

func (*FloodActionSpec) Aliases() []string {
	return []string{}
}

func (*FloodActionSpec) ShortDesc() string {
	return "network flood experiment"
}

func (d *FloodActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "network flood experiment"
}

type NetworkFloodExecutor struct {
	channel spec.Channel
}

func (*NetworkFloodExecutor) Name() string {
	return "flood"
}

var changeFloodBin = "chaos_floodnetwork"

func (ns *NetworkFloodExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"echo", "iperf"}
	if response, ok := ns.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}
	iPAddress := model.ActionFlags["ip"]
	duration := model.ActionFlags["duration"]
	port := model.ActionFlags["port"]
	parallel := model.ActionFlags["parallel"]
	if len(parallel) == 0 {
		parallel = "1"
	}
	rate := model.ActionFlags["rate"]
	err := ns.validNetworkFlood(iPAddress, port, rate, duration)
	if err != nil {
		log.Errorf(ctx, fmt.Sprintf("flood  network params error %s ", "iPAddress:"+iPAddress+","+"duration:"+duration+","+"port:"+port+","+"parallel:"+parallel+","+"rate:"+rate))
		return spec.ResponseFailWithFlags(spec.ParameterLess, err.Error())
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ns.flood(ctx)
	}
	return ns.start(ctx, iPAddress, duration, port, parallel, rate)
}

func (ns *NetworkFloodExecutor) start(ctx context.Context, iPAddress, duration, port, parallel, rate string) *spec.Response {
	nICFloodCommand := fmt.Sprintf("iperf -u -c %s -t %s -p %s -P %s -b %s", iPAddress, duration, port, parallel, rate)
	return ns.channel.Run(ctx, "", nICFloodCommand)
}

func (ns *NetworkFloodExecutor) flood(ctx context.Context) *spec.Response {
	response := new(spec.Response)
	response.Success = true
	pids, _ := process.Pids()
	for _, pid := range pids {
		pn, _ := process.NewProcess(pid)
		pName, _ := pn.Name()
		if strings.Contains(pName, "iperf") {
			proc, err := process.NewProcess(pid)
			if err != nil {
				if errors.Is(err, process.ErrorProcessNotRunning) {
					log.Errorf(ctx, "Failed to get iperf process", err.Error())
					response.Success = false
					response.Result = fmt.Sprintf("Failed to get iperf process %s", err.Error())
					return response
				}
			}
			if err := proc.Kill(); err != nil {
				log.Errorf(ctx, "the iperf process kill failed", err.Error())
				response.Success = false
				response.Result = fmt.Sprintf("Failed to get iperf process %s", err.Error())
				return response
			}
		}
	}
	return response
}

func (ns *NetworkFloodExecutor) SetChannel(channel spec.Channel) {
	ns.channel = channel
}

func (ns *NetworkFloodExecutor) validNetworkFlood(iPAddress, port, rate, duration string) error {
	if len(iPAddress) == 0 {
		return errors.New("IP address required")
	}

	if !CheckIPs(iPAddress) {
		return errors.Errorf("ip addressed %s not valid", iPAddress)
	}

	if len(port) == 0 {
		return errors.New("port is required")
	}

	if len(rate) == 0 {
		return errors.New("rate is required")
	}

	if len(duration) == 0 {
		return errors.New("duration is required")
	}
	return nil
}

func CheckIPs(i string) bool {
	if len(i) == 0 {
		return true
	}

	ips := strings.Split(i, ",")
	for _, ip := range ips {
		if !strings.Contains(ip, "/") {
			if net.ParseIP(ip) == nil {
				return false
			}
			continue
		}

		if _, _, err := net.ParseCIDR(ip); err != nil {
			return false
		}
	}

	return true
}
