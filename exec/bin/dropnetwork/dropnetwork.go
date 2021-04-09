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

package dropnetwork

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(DropNetwork))
}

type DropNetwork struct {
	DropSourceIp        string `name:"source-ip" json:"source-ip" yaml:"source-ip" default:"" help:"source ip"`
	DropDestinationIp   string `name:"destination-ip" json:"destination-ip" yaml:"destination-ip" default:"" help:"destination ip"`
	DropSourcePort      string `name:"source-port" json:"source-port" yaml:"source-port" default:"" help:"source port"`
	DropDestinationPort string `name:"destination-port" json:"destination-port" yaml:"destination-port" default:"" help:"destination port"`
	DropStringPattern   string `name:"string-pattern" json:"string-pattern" yaml:"string-pattern" default:"" help:"string pattern"`
	DropNetworkTraffic  string `name:"network-traffic" json:"network-traffic" yaml:"network-traffic" default:"" help:"network traffic"`
	DropNetStart        bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start drop"`
	DropNetStop         bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop drop"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
	StopDropNet func(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic string) `kong:"-"`
}

func (that *DropNetwork) Assign() model.Worker {
	worker := &DropNetwork{Channel: channel.NewLocalChannel()}
	worker.StopDropNet = func(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic string) {
		worker.stopDropNet(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic)
	}
	return worker
}

func (that *DropNetwork) Name() string {
	return exec.DropNetworkBin
}

func (that *DropNetwork) Exec() *spec.Response {
	if that.DropNetStart == that.DropNetStop {
		bin.PrintErrAndExit("must add --start or --stop flag")
	}
	if that.DropNetStart {
		that.startDropNet(that.DropSourceIp, that.DropDestinationIp, that.DropSourcePort, that.DropDestinationPort, that.DropStringPattern, that.DropNetworkTraffic)
	} else if that.DropNetStop {
		that.stopDropNet(that.DropSourceIp, that.DropDestinationIp, that.DropSourcePort, that.DropDestinationPort, that.DropStringPattern, that.DropNetworkTraffic)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *DropNetwork) startDropNet(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic string) {
	ctx := context.Background()
	if destinationIp == "" && sourceIp == "" && destinationPort == "" && sourcePort == "" && stringPattern == "" {
		bin.PrintErrAndExit("must specify ip or port or string flag")
		return
	}
	that.handleDropSpecifyPort(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic, ctx)
}

func (that *DropNetwork) handleDropSpecifyPort(sourceIp string, destinationIp string, sourcePort string, destinationPort string, stringPattern string, networkTraffic string, ctx context.Context) {
	var response *spec.Response
	netFlows := []string{"INPUT", "OUTPUT"}
	if networkTraffic == "in" {
		netFlows = []string{"INPUT"}
	}
	if networkTraffic == "out" {
		netFlows = []string{"OUTPUT"}
	}
	for _, netFlow := range netFlows {
		tcpArgs := fmt.Sprintf("-A %s -p tcp", netFlow)
		udpArgs := fmt.Sprintf("-A %s -p udp", netFlow)
		if sourceIp != "" {
			tcpArgs = fmt.Sprintf("%s -s %s", tcpArgs, sourceIp)
			udpArgs = fmt.Sprintf("%s -s %s", udpArgs, sourceIp)
		}
		if destinationIp != "" {
			tcpArgs = fmt.Sprintf("%s -d %s", tcpArgs, destinationIp)
			udpArgs = fmt.Sprintf("%s -d %s", udpArgs, destinationIp)
		}
		if sourcePort != "" {
			if strings.Contains(sourcePort, ","){
				tcpArgs = fmt.Sprintf("%s -m multiport --sports %s", tcpArgs, sourcePort)
				udpArgs = fmt.Sprintf("%s -m multiport --sports %s", udpArgs, sourcePort)
			} else {
				tcpArgs = fmt.Sprintf("%s --sport %s", tcpArgs, sourcePort)
				udpArgs = fmt.Sprintf("%s --sport %s", udpArgs, sourcePort)
			}
		}
		if destinationPort != "" {
			if strings.Contains(destinationPort, ","){
				tcpArgs = fmt.Sprintf("%s -m multiport --dports %s", tcpArgs, destinationPort)
				udpArgs = fmt.Sprintf("%s -m multiport --dports %s", udpArgs, destinationPort)
			}else{
				tcpArgs = fmt.Sprintf("%s --dport %s", tcpArgs, destinationPort)
				udpArgs = fmt.Sprintf("%s --dport %s", udpArgs, destinationPort)
			}
		}
		if stringPattern != "" {
			tcpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", tcpArgs, stringPattern)
			udpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", udpArgs, stringPattern)
		}
		tcpArgs = fmt.Sprintf("%s -j DROP", tcpArgs)
		udpArgs = fmt.Sprintf("%s -j DROP", udpArgs)
		response = that.Channel.Run(ctx, "iptables", fmt.Sprintf(`%s`, tcpArgs))
		if !response.Success {
			that.StopDropNet(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic)
			bin.PrintErrAndExit(response.Err)
			return
		}
		response = that.Channel.Run(ctx, "iptables", fmt.Sprintf(`%s`, udpArgs))
		if !response.Success {
			that.StopDropNet(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic)
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *DropNetwork) stopDropNet(sourceIp, destinationIp, sourcePort, destinationPort, stringPattern, networkTraffic string) {
	ctx := context.Background()
	var response *spec.Response
	netFlows := []string{"INPUT", "OUTPUT"}
	if networkTraffic == "in" {
		netFlows = []string{"INPUT"}
	}
	if networkTraffic == "out" {
		netFlows = []string{"OUTPUT"}
	}
	for _, netFlow := range netFlows {
		tcpArgs := fmt.Sprintf("-D %s -p tcp", netFlow)
		udpArgs := fmt.Sprintf("-D %s -p udp", netFlow)
		if sourceIp != "" {
			tcpArgs = fmt.Sprintf("%s -s %s", tcpArgs, sourceIp)
			udpArgs = fmt.Sprintf("%s -s %s", udpArgs, sourceIp)
		}
		if destinationIp != "" {
			tcpArgs = fmt.Sprintf("%s -d %s", tcpArgs, destinationIp)
			udpArgs = fmt.Sprintf("%s -d %s", udpArgs, destinationIp)
		}
		if sourcePort != "" {
			if strings.Contains(sourcePort, ","){
				tcpArgs = fmt.Sprintf("%s -m multiport --sports %s", tcpArgs, sourcePort)
				udpArgs = fmt.Sprintf("%s -m multiport --sports %s", udpArgs, sourcePort)
			} else {
				tcpArgs = fmt.Sprintf("%s --sport %s", tcpArgs, sourcePort)
				udpArgs = fmt.Sprintf("%s --sport %s", udpArgs, sourcePort)
			}
		}
		if destinationPort != "" {
			if strings.Contains(destinationPort, ","){
				tcpArgs = fmt.Sprintf("%s -m multiport --dports %s", tcpArgs, destinationPort)
				udpArgs = fmt.Sprintf("%s -m multiport --dports %s", udpArgs, destinationPort)
			}else{
				tcpArgs = fmt.Sprintf("%s --dport %s", tcpArgs, destinationPort)
				udpArgs = fmt.Sprintf("%s --dport %s", udpArgs, destinationPort)
			}
		}
		if stringPattern != "" {
			tcpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", tcpArgs, stringPattern)
			udpArgs = fmt.Sprintf("%s -m string --string %s --algo bm", udpArgs, stringPattern)
		}
		tcpArgs = fmt.Sprintf("%s -j DROP", tcpArgs)
		udpArgs = fmt.Sprintf("%s -j DROP", udpArgs)
		response = that.Channel.Run(ctx, "iptables", fmt.Sprintf(`%s`, tcpArgs))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
		response = that.Channel.Run(ctx, "iptables", fmt.Sprintf(`%s`, udpArgs))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
