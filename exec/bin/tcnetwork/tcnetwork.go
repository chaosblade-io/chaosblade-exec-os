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

package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	channel2 "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var tcNetInterface, tcLocalPort, tcRemotePort, tcExcludePort string
var tcDestinationIp string
var netPercent, delayNetTime, delayNetOffset string
var tcNetStart, tcNetStop bool
var tcIgnorePeerPorts bool
var actionType string
var reorderGap string
var correlation string

const delimiter = ","
const (
	Delay     = "delay"
	Loss      = "loss"
	Duplicate = "duplicate"
	Corrupt   = "corrupt"
	Reorder   = "reorder"
)

func main() {
	flag.StringVar(&tcNetInterface, "interface", "", "network interface")
	flag.StringVar(&delayNetTime, "time", "", "delay time")
	flag.StringVar(&delayNetOffset, "offset", "", "delay offset")
	flag.StringVar(&netPercent, "percent", "", "loss percent")
	flag.StringVar(&tcLocalPort, "local-port", "", "local ports, for example: 80,8080,8081")
	flag.StringVar(&tcRemotePort, "remote-port", "", "remote ports, for example: 80,8080,8081")
	flag.StringVar(&tcExcludePort, "exclude-port", "", "exclude ports, for example: 22,23")
	flag.StringVar(&tcDestinationIp, "destination-ip", "", "destination ip")
	flag.BoolVar(&tcNetStart, "start", false, "start delay")
	flag.BoolVar(&tcNetStop, "stop", false, "stop delay")
	flag.BoolVar(&tcIgnorePeerPorts, "ignore-peer-port", false, "ignore excluding all ports communicating with this port, generally used when the ss command does not exist")
	flag.StringVar(&actionType, "type", "", "network experiment type, value is delay|loss|duplicate|corrupt|reorder, required")
	flag.StringVar(&reorderGap, "gap", "", "packets gap")
	flag.StringVar(&correlation, "correlation", "0", "correlation on previous packet")
	bin.ParseFlagAndInitLog()

	if tcNetInterface == "" {
		bin.PrintErrAndExit("less --interface flag")
	}

	if tcNetStart {
		var classRule string
		switch actionType {
		case Delay:
			classRule = fmt.Sprintf("netem delay %sms %sms", delayNetTime, delayNetOffset)
		case Loss:
			classRule = fmt.Sprintf("netem loss %s%%", netPercent)
		case Duplicate:
			classRule = fmt.Sprintf("netem duplicate %s%%", netPercent)
		case Corrupt:
			classRule = fmt.Sprintf("netem corrupt %s%%", netPercent)
		case Reorder:
			classRule = fmt.Sprintf("netem reorder %s%% %s%%", netPercent, correlation)
			if reorderGap != "" {
				classRule = fmt.Sprintf("%s gap %s", classRule, reorderGap)
			}
			classRule = fmt.Sprintf("%s delay %sms", classRule, delayNetTime)
		default:
			bin.PrintErrAndExit("unsupported type for network experiments")
		}
		startNet(tcNetInterface, classRule, tcLocalPort, tcRemotePort, tcExcludePort, tcDestinationIp)
	} else if tcNetStop {
		stopNet(tcNetInterface)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var channel = channel2.NewLocalChannel()

func startNet(netInterface, classRule, localPort, remotePort, excludePort, destIp string) {
	// check device txqueuelen size, if the size is zero, then set the value to 1000
	response := preHandleTxqueue(netInterface)
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
	}
	ctx := context.Background()
	// assert localPort and remotePort
	if localPort == "" && remotePort == "" && excludePort == "" && destIp == "" {
		response := channel.Run(ctx, "tc", fmt.Sprintf(`qdisc add dev %s root %s`, netInterface, classRule))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
	response = addQdiscForDL(channel, ctx, netInterface)
	// only ip
	if localPort == "" && remotePort == "" && excludePort == "" {
		response = addIpFilterForDL(ctx, channel, netInterface, classRule, destIp)
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
	ipRule := getIpRule(destIp)
	// exclude
	if localPort == "" && remotePort == "" && excludePort != "" {
		response = addExcludePortFilterForDL(ctx, channel, netInterface, classRule, excludePort, ipRule)
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
	// local port or remote port
	response = addLocalOrRemotePortForDL(ctx, channel, netInterface, classRule, localPort, remotePort, ipRule)
	bin.PrintOutputAndExit(response.Result.(string))
}

func preHandleTxqueue(netInterface string) *spec.Response {
	txFile := fmt.Sprintf("/sys/class/net/%s/tx_queue_len", netInterface)
	isExist := util.IsExist(txFile)
	if isExist {
		// check the value
		response := channel.Run(context.TODO(), "head", fmt.Sprintf("-1 %s", txFile))
		if response.Success {
			txlen := strings.TrimSpace(response.Result.(string))
			len, err := strconv.Atoi(txlen)
			if err != nil {
				logrus.Warningf("parse %s file err, %v", txFile, err)
			} else {
				if len > 0 {
					return response
				} else {
					logrus.Infof("the tx_queue_len value for %s is %s", netInterface, txlen)
				}
			}
		}
	}
	// set to 1000 directly
	response := channel.Run(context.TODO(), "ifconfig", fmt.Sprintf("%s txqueuelen 1000", netInterface))
	if !response.Success {
		logrus.Warningf("set txqueuelen for %s err, %s", netInterface, response.Err)
	}
	return response
}

func getIpRule(destIp string) string {
	if destIp == "" {
		return ""
	}
	return fmt.Sprintf("match ip dst %s", destIp)
}

// addIpFilterForDL
func addIpFilterForDL(ctx context.Context, channel spec.Channel, netInterface string, classRule, destIp string) *spec.Response {
	response := channel.Run(ctx, "tc",
		fmt.Sprintf(`qdisc add dev %s parent 1:4 handle 40: %s`, netInterface, classRule))
	if !response.Success {
		stopNet(netInterface)
		bin.PrintErrAndExit(response.Err)
		return response
	}
	args := fmt.Sprintf(
		`filter add dev %s parent 1: prio 4 protocol ip u32 match ip dst %s flowid 1:4`,
		netInterface, destIp)
	response = channel.Run(ctx, "tc", args)
	if !response.Success {
		stopDLNetFunc(netInterface)
		bin.PrintErrAndExit(response.Err)
	}
	return response
}

var stopDLNetFunc = stopNet

// addLocalOrRemotePortForDL creates class rule in 1:4 queue and add filter to the queue
func addLocalOrRemotePortForDL(ctx context.Context, channel spec.Channel,
	netInterface, classRule, localPort, remotePort, ipRule string) *spec.Response {
	response := channel.Run(ctx, "tc",
		fmt.Sprintf(`qdisc add dev %s parent 1:4 handle 40: %s`, netInterface, classRule))
	if !response.Success {
		stopNet(netInterface)
		bin.PrintErrAndExit(response.Err)
		return response
	}
	// local port 0
	if localPort != "" {
		ports := strings.Split(localPort, delimiter)
		args := fmt.Sprintf(
			`filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip sport %s 0xffff flowid 1:4`,
			netInterface, ipRule, ports[0])
		if len(ports) > 1 {
			for i := 1; i < len(ports); i++ {
				args = fmt.Sprintf(
					`%s && \
					tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip sport %s 0xffff flowid 1:4`,
					args, netInterface, ipRule, ports[i])
			}
		}
		response = channel.Run(ctx, "tc", args)
		if !response.Success {
			stopDLNetFunc(netInterface)
			bin.PrintErrAndExit(response.Err)
		}
	}
	// remote port 2
	if remotePort != "" {
		ports := strings.Split(remotePort, delimiter)
		args := fmt.Sprintf(
			`filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip dport %s 0xffff flowid 1:4`,
			netInterface, ipRule, ports[0])
		if len(ports) > 1 {
			for i := 1; i < len(ports); i++ {
				args = fmt.Sprintf(
					`%s && \
					tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip dport %s 0xffff flowid 1:4`,
					args, netInterface, ipRule, ports[i])
			}
		}
		response = channel.Run(ctx, "tc", args)
		if !response.Success {
			stopDLNetFunc(netInterface)
			bin.PrintErrAndExit(response.Err)
		}
	}
	return response
}

// addExcludePortFilterForDL create class rule for each band and add filter to 1:4
func addExcludePortFilterForDL(ctx context.Context, channel spec.Channel,
	netInterface, classRule, excludePort, ipRule string) *spec.Response {
	args := fmt.Sprintf(
		`qdisc add dev %s parent 1:1 %s && \
			tc qdisc add dev %s parent 1:2 %s && \
			tc qdisc add dev %s parent 1:3 %s && \
			tc qdisc add dev %s parent 1:4 handle 40: pfifo_fast`,
		netInterface, classRule, netInterface, classRule, netInterface, classRule, netInterface)
	ports := strings.Split(excludePort, delimiter)

	// add peer ports
	portSet := make(map[string]interface{}, 0)
	for _, p := range ports {
		if _, ok := portSet[p]; !ok {
			portSet[p] = struct{}{}
		}
		if !tcIgnorePeerPorts {
			peerPorts, err := getPeerPorts(p)
			if err != nil {
				logrus.Warningf("get peer ports for %s err, %v", p, err)
				errMsg := fmt.Sprintf("get peer ports for %s err, %v, please solve the problem or skip to exclude peer ports by --ignore-peer-port flag", p, err)
				stopDLNetFunc(netInterface)
				bin.PrintErrAndExit(errMsg)
				return spec.ReturnFail(spec.Code[spec.ExecCommandError], errMsg)
			}
			logrus.Infof("peer ports for %s: %v", p, peerPorts)
			for _, mp := range peerPorts {
				if _, ok := portSet[mp]; ok {
					continue
				}
				portSet[mp] = struct{}{}
			}
		}
	}
	for k := range portSet {
		args = fmt.Sprintf(
			`%s && \
			tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip sport %s 0xffff flowid 1:4 && \
			tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip dport %s 0xffff flowid 1:4`,
			args, netInterface, ipRule, k, netInterface, ipRule, k)
	}
	response := channel.Run(ctx, "tc", args)
	if !response.Success {
		stopDLNetFunc(netInterface)
		bin.PrintErrAndExit(response.Err)
		return response
	}
	return response
}

// addQdiscForDL creates bands for filter
func addQdiscForDL(channel spec.Channel, ctx context.Context, netInterface string) *spec.Response {
	// add tc filter for delay specify port
	response := channel.Run(ctx, "tc", fmt.Sprintf(`qdisc add dev %s root handle 1: prio bands 4`, netInterface))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return response
	}
	return response
}

// stopNet, no need to add os.Exit
func stopNet(netInterface string) {
	ctx := context.Background()
	channel.Run(ctx, "tc", fmt.Sprintf(`filter del dev %s parent 1: prio 4`, netInterface))
	channel.Run(ctx, "tc", fmt.Sprintf(`qdisc del dev %s root`, netInterface))
}

// getPeerPorts returns all ports communicating with the port
func getPeerPorts(port string) ([]string, error) {
	response := channel.Run(context.TODO(), "command", "-v ss")
	if !response.Success {
		return nil, fmt.Errorf("ss command not found")
	}
	response = channel.Run(context.TODO(), "ss", fmt.Sprintf("-n sport = %s or dport = %s", port, port))
	if !response.Success {
		return nil, fmt.Errorf(response.Err)
	}
	if util.IsNil(response.Result) {
		return []string{}, nil
	}
	result := response.Result.(string)
	ssMsg := strings.TrimSpace(result)
	if ssMsg == "" {
		return []string{}, nil
	}
	sockets := strings.Split(ssMsg, "\n")
	logrus.Infof("sockets for %s, %v", port, sockets)
	mappingPorts := make([]string, 0)
	for idx, s := range sockets {
		if idx == 0 {
			continue
		}
		fields := strings.Fields(s)
		for _, f := range fields {
			if !strings.Contains(f, ":") {
				continue
			}
			ipPort := strings.Split(f, ":")
			if len(ipPort) != 2 {
				logrus.Warningf("illegal socket address: %s", f)
				continue
			}
			mappingPorts = append(mappingPorts, ipPort[1])
		}
	}
	return mappingPorts, nil
}
