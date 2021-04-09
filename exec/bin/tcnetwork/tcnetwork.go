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

package tcnetwork

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(TCNetwork))
}

type TCNetwork struct {
	TcNetInterface    string `name:"interface" json:"interface" yaml:"interface" default:"" help:"network interface"`
	DelayNetTime      string `name:"time" json:"time" yaml:"time" default:"" help:"delay time"`
	DelayNetOffset    string `name:"offset" json:"offset" yaml:"offset" default:"" help:"delay offset"`
	NetPercent        string `name:"percent" json:"percent" yaml:"percent" default:"" help:"loss percent"`
	TcLocalPort       string `name:"local-port" json:"local-port" yaml:"local-port" default:"" help:"local ports, for example: 80,8080,8081"`
	TcRemotePort      string `name:"remote-port" json:"remote-port" yaml:"remote-port" default:"" help:"remote ports, for example: 80,8080,8081"`
	TcExcludePort     string `name:"exclude-port" json:"exclude-port" yaml:"exclude-port" default:"" help:"exclude ports, for example: 22,23"`
	TcDestinationIp   string `name:"destination-ip" json:"destination-ip" yaml:"destination-ip" default:"" help:"destination ip"`
	TcExcludeIp       string `name:"exclude-ip" json:"exclude-ip" yaml:"exclude-ip" default:"" help:"exclude ip"`
	TcNetStart        bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start delay"`
	TcNetStop         bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop delay"`
	TcIgnorePeerPorts bool   `name:"ignore-peer-port" json:"ignore-peer-port" yaml:"ignore-peer-port" default:"false" help:"ignore excluding all ports communicating with this port, generally used when the ss command does not exist"`
	ActionType        string `name:"type" json:"type" yaml:"type" default:"" help:"network experiment type, value is delay|loss|duplicate|corrupt|reorder, required"`
	ReorderGap        string `name:"gap" json:"gap" yaml:"gap" default:"" help:"packets gap"`
	Correlation       string `name:"correlation" json:"correlation" yaml:"correlation" default:"0" help:"correlation on previous packet"`
	TcForce           bool   `name:"force" json:"force" yaml:"force" default:"false" help:"forcibly overwrites the original rules"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
	StopNet func(netInterface string) `kong:"-"`
}

func (that *TCNetwork) Assign() model.Worker {
	worker := &TCNetwork{Channel: channel.NewLocalChannel()}
	worker.StopNet = func(netInterface string) {
		worker.stopNet(netInterface)
	}
	return worker
}

func (that *TCNetwork) Name() string {
	return exec.TcNetworkBin
}

func (that *TCNetwork) Exec() *spec.Response {
	if that.TcNetInterface == "" {
		bin.PrintErrAndExit("less --interface flag")
	}

	if that.TcNetStart {
		var classRule string
		switch that.ActionType {
		case Delay:
			classRule = fmt.Sprintf("netem delay %sms %sms", that.DelayNetTime, that.DelayNetOffset)
		case Loss:
			classRule = fmt.Sprintf("netem loss %s%%", that.NetPercent)
		case Duplicate:
			classRule = fmt.Sprintf("netem duplicate %s%%", that.NetPercent)
		case Corrupt:
			classRule = fmt.Sprintf("netem corrupt %s%%", that.NetPercent)
		case Reorder:
			classRule = fmt.Sprintf("netem reorder %s%% %s%%", that.NetPercent, that.Correlation)
			if that.ReorderGap != "" {
				classRule = fmt.Sprintf("%s gap %s", classRule, that.ReorderGap)
			}
			classRule = fmt.Sprintf("%s delay %sms", classRule, that.DelayNetTime)
		default:
			bin.PrintErrAndExit("unsupported type for network experiments")
		}
		that.startNet(that.TcNetInterface, classRule, that.TcLocalPort, that.TcRemotePort, that.TcExcludePort, that.TcDestinationIp, that.TcExcludeIp, that.TcForce)
	} else if that.TcNetStop {
		that.stopNet(that.TcNetInterface)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

const delimiter = ","
const (
	Delay     = "delay"
	Loss      = "loss"
	Duplicate = "duplicate"
	Corrupt   = "corrupt"
	Reorder   = "reorder"
)

func (that *TCNetwork) startNet(netInterface, classRule, localPort, remotePort, excludePort, destIp, excludeIp string, force bool) {
	// check device txqueuelen size, if the size is zero, then set the value to 1000
	response := that.preHandleTxqueue(netInterface)
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
	}
	ips, err := readServerIps()
	if len(ips) > 0 {
		channelIps := strings.Join(ips, ",")
		if excludeIp != "" {
			excludeIp = fmt.Sprintf("%s,%s", channelIps, excludeIp)
		} else {
			excludeIp = channelIps
		}
	}
	if force {
		that.stopNet(netInterface)
	}
	ctx := context.Background()
	// Only interface flag
	if localPort == "" && remotePort == "" && excludePort == "" && destIp == "" && excludeIp == "" {
		response := that.Channel.Run(ctx, "tc", fmt.Sprintf(`qdisc add dev %s root %s`, netInterface, classRule))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
	response = addQdiscForDL(that.Channel, ctx, netInterface)

	var excludePorts []string
	if excludePort != "" {
		excludePorts, err = that.getExcludePorts(excludePort)
		if err != nil {
			that.StopNet(netInterface)
			bin.PrintErrAndExit(response.Err)
		}
	}

	// only contains excludePort or excludeIP
	if localPort == "" && remotePort == "" && destIp == "" {
		// Add class rule to 1,2,3 band, exclude port and exclude ip are added to 4 band
		args := buildNetemToDefaultBandsArgs(netInterface, classRule)
		excludeFilters := that.buildExcludeFilterToNewBand(netInterface, excludePorts, excludeIp)
		response := that.Channel.Run(ctx, "tc", args+excludeFilters)
		if !response.Success {
			that.StopNet(netInterface)
			bin.PrintErrAndExit(response.Err)
		}
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
	destIpRules := that.getIpRules(destIp)
	excludeIpRules := that.getIpRules(excludeIp)
	// local port or remote port
	response = that.executeTargetPortAndIpWithExclude(ctx, that.Channel, netInterface, classRule, localPort, remotePort, destIpRules,
		excludePorts, excludeIpRules)
	bin.PrintOutputAndExit(response.Result.(string))
}

func (that *TCNetwork) getExcludePorts(excludePort string) ([]string, error) {
	ports := strings.Split(excludePort, delimiter)

	// add peer ports
	portSet := make(map[string]interface{}, 0)
	for _, p := range ports {
		if _, ok := portSet[p]; !ok {
			portSet[p] = struct{}{}
		}
		if !that.TcIgnorePeerPorts {
			peerPorts, err := that.getPeerPorts(p)
			if err != nil {
				logrus.Warningf("get peer ports for %s err, %v", p, err)
				errMsg := fmt.Sprintf("get peer ports for %s err, %v, please solve the problem or skip to exclude peer ports by --ignore-peer-port flag", p, err)
				return nil, fmt.Errorf(errMsg)
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
	excludePorts := make([]string, 0)
	for k := range portSet {
		excludePorts = append(excludePorts, k)
	}
	return excludePorts, nil
}

func (that *TCNetwork) buildExcludeFilterToNewBand(netInterface string, excludePorts []string, excludeIp string) string {
	var args string
	excludeIpRules := that.getIpRules(excludeIp)
	for _, rule := range excludeIpRules {
		args = fmt.Sprintf(
			`%s && \
			tc filter add dev %s parent 1: prio 4 protocol ip u32 %s flowid 1:4`,
			args, netInterface, rule)
	}

	for _, port := range excludePorts {
		if strings.TrimSpace(port) == "" {
			continue
		}
		args = fmt.Sprintf(
			`%s && \
			tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip sport %s 0xffff flowid 1:4 && \,
			tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip sport %s 0xffff flowid 1:4`,
			args, netInterface, port, netInterface, port)
	}
	return args
}

func buildNetemToDefaultBandsArgs(netInterface, classRule string) string {
	args := fmt.Sprintf(
		`qdisc add dev %s parent 1:1 %s && \
			tc qdisc add dev %s parent 1:2 %s && \
			tc qdisc add dev %s parent 1:3 %s && \
			tc qdisc add dev %s parent 1:4 handle 40: prio`,
		netInterface, classRule, netInterface, classRule, netInterface, classRule, netInterface)
	return args
}

// Reserved for the peer server ips of the command channel
func readServerIps() ([]string, error) {
	ips := make([]string, 0)
	return ips, nil
}

func (that *TCNetwork) preHandleTxqueue(netInterface string) *spec.Response {
	txFile := fmt.Sprintf("/sys/class/net/%s/tx_queue_len", netInterface)
	isExist := util.IsExist(txFile)
	if isExist {
		// check the value
		response := that.Channel.Run(context.TODO(), "head", fmt.Sprintf("-1 %s", txFile))
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
	if that.Channel.IsCommandAvailable("ifconfig") {
		// set to 1000 directly
		response := that.Channel.Run(context.TODO(), "ifconfig", fmt.Sprintf("%s txqueuelen 1000", netInterface))
		if !response.Success {
			logrus.Warningf("set txqueuelen for %s err, %s", netInterface, response.Err)
		}
		return response
	}
	return spec.ReturnSuccess("success")
}

func (that *TCNetwork) getIpRules(targetIp string) []string {
	if targetIp == "" {
		return []string{}
	}
	ipString := strings.TrimSpace(targetIp)
	ips := strings.Split(ipString, delimiter)
	ipRules := make([]string, 0)
	for _, ip := range ips {
		if strings.TrimSpace(ip) == "" {
			continue
		}
		ipRules = append(ipRules, fmt.Sprintf("match ip dst %s", ip))
	}
	return ipRules
}


// executeTargetPortAndIpWithExclude creates class rule in 1:4 queue and add filter to the queue
func (that *TCNetwork) executeTargetPortAndIpWithExclude(ctx context.Context, channel spec.Channel,
	netInterface, classRule, localPort, remotePort string, destIpRules, excludePorts, excludeIpRules []string) *spec.Response {
	args := fmt.Sprintf(`qdisc add dev %s parent 1:4 handle 40: %s`, netInterface, classRule)
	args = buildTargetFilterPortAndIp(localPort, remotePort, destIpRules, excludePorts, excludeIpRules, args, netInterface)
	response := channel.Run(ctx, "tc", args)
	if !response.Success {
		that.StopNet(netInterface)
		bin.PrintErrAndExit(response.Err)
	}
	return response
}

func buildTargetFilterPortAndIp(localPort, remotePort string, destIpRules, excludePorts, excludeIpRules []string, args string, netInterface string) string {
	if localPort != "" {
		ports := strings.Split(localPort, delimiter)
		for _, port := range ports {
			if len(destIpRules) > 0 {
				for _, ipRule := range destIpRules {
					args = fmt.Sprintf(
						`%s && \
						tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip sport %s 0xffff flowid 1:4`,
						args, netInterface, ipRule, port)
				}
			} else {
				args = fmt.Sprintf(
					`%s && \
					tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip sport %s 0xffff flowid 1:4`,
					args, netInterface, port)
			}
		}
	}
	if remotePort != "" {
		ports := strings.Split(remotePort, delimiter)
		for _, port := range ports {
			if len(destIpRules) > 0 {
				for _, ipRule := range destIpRules {
					args = fmt.Sprintf(
						`%s && \
						tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip dport %s 0xffff flowid 1:4`,
						args, netInterface, ipRule, port)
				}
			} else {
				args = fmt.Sprintf(
					`%s && \
					tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip dport %s 0xffff flowid 1:4`,
					args, netInterface, port)
			}
		}
	}
	if remotePort == "" && localPort == "" {
		// only destIp
		for _, ipRule := range destIpRules {
			args = fmt.Sprintf(
				`%s && \
				tc filter add dev %s parent 1: prio 4 protocol ip u32 %s flowid 1:4`,
				args, netInterface, ipRule)
		}
	}
	if len(excludeIpRules) > 0 {
		for _, ipRule := range excludeIpRules {
			args = fmt.Sprintf(
				`%s && \
				tc filter add dev %s parent 1: prio 3 protocol ip u32 %s flowid 1:3`,
				args, netInterface, ipRule)
		}
	}
	if len(excludePorts) > 0 {
		for _, port := range excludePorts {
			args = fmt.Sprintf(
				`%s && \
				tc filter add dev %s parent 1: prio 3 protocol ip u32 match ip dport %s 0xffff flowid 1:3 && \
				tc filter add dev %s parent 1: prio 3 protocol ip u32 match ip sport %s 0xffff flowid 1:3`,
				args, netInterface, port, netInterface, port)
		}
	}
	return args
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
func (that *TCNetwork) stopNet(netInterface string) {
	ctx := context.Background()
	that.Channel.Run(ctx, "tc", fmt.Sprintf(`filter del dev %s parent 1: prio 4`, netInterface))
	that.Channel.Run(ctx, "tc", fmt.Sprintf(`qdisc del dev %s root`, netInterface))
}

// getPeerPorts returns all ports communicating with the port
func (that *TCNetwork) getPeerPorts(port string) ([]string, error) {
	response := that.Channel.Run(context.TODO(), "command", "-v ss")
	if !response.Success {
		return nil, fmt.Errorf("ss command not found")
	}
	response = that.Channel.Run(context.TODO(), "ss", fmt.Sprintf("-n sport = %s or dport = %s", port, port))
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
