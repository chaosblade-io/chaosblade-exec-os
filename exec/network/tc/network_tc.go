/*
 * Copyright 2025 The ChaosBlade Authors
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
	"errors"
	"fmt"
	"math"
	"math/bits"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
)

// TcNetworkBin for network delay, loss, duplicate, reorder and corrupt experiments
const TcNetworkBin = "chaos_tcnetwork"

// tcName is the system tc command name, used as the fallback when no bundled
// tc is shipped alongside chaos_os.
const tcName = "tc"

// bundledTcPath returns the absolute path of the tc binary shipped together
// with chaos_os. It lives in the same directory as the chaos_os executable
// (util.GetProgramPath), e.g. /opt/chaosblade/bin/tc inside the tool image,
// exactly like the bundled strace.
//
// Bundling a pinned, statically linked iproute2 (<=6.8) tc removes the
// dependency on whatever iproute2 the base image happens to ship. Since
// iproute2 v6.9.0 the tc netem encoder writes the 64-bit TCA_NETEM_LATENCY64
// attribute and leaves the 32-bit opt.latency zero; kernels older than 4.15 do
// not understand LATENCY64, read opt.latency (0) and silently ignore the
// unknown attribute, so `network delay` is created but has no effect.
func bundledTcPath() string {
	return path.Join(util.GetProgramPath(), tcName)
}

// hasBundledTc reports whether the bundled tc is present.
func hasBundledTc() bool {
	return util.IsExist(bundledTcPath())
}

// tcCommand returns the shell command used to invoke tc. When the bundled tc is
// present it prepends the bundle directory to PATH so that BOTH the leading tc
// and every tc embedded in compound args (e.g. `&& tc filter add ...`) resolve
// to the pinned bundled binary; otherwise it falls back to the plain system tc.
//
// This works for every channel: both LocalChannel and NSExecChannel join the
// script and args into a single `/bin/sh -c "<script> <args>"` string, and the
// nsexec path does not enter the mount namespace, so the absolute bundle
// directory is resolved in the tool container filesystem while the netns being
// configured is still the target's.
func tcCommand() string {
	if hasBundledTc() {
		return fmt.Sprintf("export PATH=%s:$PATH; tc", util.GetProgramPath())
	}
	return tcName
}

// tcAvailabilityCommands returns the command list for the pre-run availability
// check. When the bundled tc is present the system tc on PATH is irrelevant
// (execution uses the bundle directory, which is not on the default PATH), so
// it is not checked; otherwise the fallback still requires the system tc.
func tcAvailabilityCommands() []string {
	if hasBundledTc() {
		return []string{"head"}
	}
	return []string{tcName, "head"}
}

// netemKeywords extracts the netem attribute keywords (delay/loss/duplicate/
// corrupt/reorder) from a class rule such as "netem delay 3000ms 0ms".
//
// Only attributes the user asked to be non-zero are returned. tc omits an
// attribute from `qdisc show` when its value is zero (the printer guards each
// one, e.g. `if (qopt.latency)`), so verifying a zero-valued delay/loss would
// always fail the read-back and wrongly roll back a legitimate injection.
func netemKeywords(classRule string) []string {
	supported := []string{"delay", "loss", "duplicate", "corrupt", "reorder"}
	keywords := make([]string, 0)
	fields := strings.Fields(classRule)
	for i, field := range fields {
		for _, kw := range supported {
			if field != kw {
				continue
			}
			// The value token follows the keyword, e.g. "3000ms" or "10%".
			if i+1 < len(fields) && isNonZeroValue(fields[i+1]) {
				keywords = append(keywords, kw)
			}
		}
	}
	return keywords
}

// isNonZeroValue reports whether a netem attribute value token such as "3000ms"
// or "10%" represents a non-zero amount. An unparseable token is treated as
// non-zero so the attribute is still verified (fail towards catching the bug).
func isNonZeroValue(token string) bool {
	trimmed := strings.TrimRight(token, "ms%")
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return true
	}
	return value > 0
}

// verifyNetemApplied reads the qdisc back to confirm the netem attribute was
// really accepted by the kernel. tc exits 0 even when the kernel silently
// ignores an attribute it does not understand (see bundledTcPath), which would
// otherwise leave `network delay` looking applied while having no effect. When
// the expected attribute is missing the injection is reported as a failure so
// the caller can roll back instead of silently succeeding.
func verifyNetemApplied(ctx context.Context, cl spec.Channel, netInterface, classRule string) *spec.Response {
	keywords := netemKeywords(classRule)
	if len(keywords) == 0 {
		return spec.ReturnSuccess("no netem attribute to verify")
	}
	response := cl.Run(ctx, tcCommand(), fmt.Sprintf(`qdisc show dev %s`, netInterface))
	if !response.Success {
		// A failed read-back must not mask an otherwise successful injection
		// (e.g. a restricted environment); warn and let it pass.
		log.Warnf(ctx, "verify netem: read qdisc for %s failed, %v", netInterface, response.Err)
		return spec.ReturnSuccess(response.Result)
	}
	output := ""
	if response.Result != nil {
		output, _ = response.Result.(string)
	}
	for _, kw := range keywords {
		if !strings.Contains(output, kw) {
			return spec.ReturnFail(spec.UnexpectedStatus, fmt.Sprintf(
				"netem %s did not take effect on %s: the qdisc was created but the kernel dropped the %q attribute (readback: %s). This happens when the tc/iproute2 version writes an attribute the target kernel does not understand; ship the bundled compatible tc (iproute2<=6.8)",
				kw, netInterface, kw, strings.TrimSpace(output)))
		}
	}
	return spec.ReturnSuccess(output)
}

var commFlags = []spec.ExpFlagSpec{
	&spec.ExpFlag{
		Name: "local-port",
		Desc: "Ports for local service. Support for configuring multiple ports, separated by commas or connector representing ranges, for example: 80,8000-8080",
	},
	&spec.ExpFlag{
		Name: "remote-port",
		Desc: "Ports for remote service. Support for configuring multiple ports, separated by commas or connector representing ranges, for example: 80,8000-8080",
	},
	&spec.ExpFlag{
		Name: "exclude-port",
		Desc: "Exclude local ports. Support for configuring multiple ports, separated by commas or connector representing ranges, for example: 22,8000. This flag is invalid when --local-port or --remote-port is specified",
	},
	&spec.ExpFlag{
		Name: "destination-ip",
		Desc: "destination ip. Support for using mask to specify the ip range such as 92.168.1.0/24 or comma separated multiple ips, for example 10.0.0.1,11.0.0.1.",
	},
	&spec.ExpFlag{
		Name:   "ignore-peer-port",
		Desc:   "ignore excluding all ports communicating with this port, generally used when the ss command does not exist",
		NoArgs: true,
	},
	&spec.ExpFlag{
		Name:                  "interface",
		Desc:                  "Network interface, for example, eth0",
		Required:              true,
		RequiredWhenDestroyed: true,
	},
	&spec.ExpFlag{
		Name: "exclude-ip",
		Desc: "Exclude ips. Support for using mask to specify the ip range such as 92.168.1.0/24 or comma separated multiple ips, for example 10.0.0.1,11.0.0.1",
	},
	&spec.ExpFlag{
		Name: "protocol",
		Desc: "specify protocol for example tcp udp icmp ",
	},
	&spec.ExpFlag{
		Name:   "force",
		Desc:   "Forcibly overwrites the original rules",
		NoArgs: true,
	},
}

const delimiter = ","

func startNet(ctx context.Context, netInterface, classRule, localPort, remotePort, excludePort, destIp, excludeIp string, force, ignorePeerPorts bool, protocol string, cl spec.Channel) *spec.Response {
	if protocol != "" {
		switch protocol {
		case "tcp":
			protocol = "6"
		case "udp":
			protocol = "17"
		case "icmp":
			protocol = "1"
		default:
			return spec.ResponseFailWithFlags(spec.ParameterInvalid, "protocol", protocol, "unsupport protocol")
		}
	}
	var localPortRanges, remotePortRanges, excludePortRanges [][]int
	var err error
	if localPort != "" {
		localPortRanges, err = parseIntegerListToPortRanges("local-port", localPort)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "local-port", localPort, err)
		}
	}
	if remotePort != "" {
		remotePortRanges, err = parseIntegerListToPortRanges("remote-port", remotePort)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "remote-port", remotePort, err)
		}
	}
	if excludePort != "" {
		excludePortRanges, err = getExcludePortRanges(ctx, excludePort, ignorePeerPorts, cl)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "exclude-port", excludePort, err)
		}
	}

	// check device txqueuelen size, if the size is zero, then set the value to 1000
	response := preHandleTxqueue(ctx, netInterface, cl)
	if !response.Success {
		return response
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
		stopNet(ctx, netInterface, cl)
	}
	// Only interface flag
	if localPort == "" && remotePort == "" && excludePort == "" && destIp == "" && excludeIp == "" && protocol == "" {
		response := cl.Run(ctx, tcCommand(), fmt.Sprintf(`qdisc add dev %s root %s`, netInterface, classRule))
		if !response.Success {
			return response
		}
		if verifyResp := verifyNetemApplied(ctx, cl, netInterface, classRule); !verifyResp.Success {
			stopNet(ctx, netInterface, cl)
			return verifyResp
		}
		return response
	}

	response = addQdiscForDL(cl, ctx, netInterface)

	// only contains excludePort or excludeIP
	if localPort == "" && remotePort == "" && destIp == "" && protocol == "" {
		// Add class rule to 1,2,3 band, exclude port and exclude ip are added to 4 band
		args := buildNetemToDefaultBandsArgs(netInterface, classRule)
		excludeFilters := buildExcludeFilterToNewBand(netInterface, excludePortRanges, excludeIp)
		response := cl.Run(ctx, tcCommand(), args+excludeFilters)
		if !response.Success {
			stopNet(ctx, netInterface, cl)
			return response
		}
		if verifyResp := verifyNetemApplied(ctx, cl, netInterface, classRule); !verifyResp.Success {
			stopNet(ctx, netInterface, cl)
			return verifyResp
		}
		return response
	}
	destIpRules := getIpRules(destIp)
	excludeIpRules := getExcludeIpRules(excludeIp)
	// local port or remote port
	return executeTargetPortAndIpWithExclude(ctx, cl, netInterface, classRule, localPortRanges, remotePortRanges, destIpRules,
		excludePortRanges, excludeIpRules, protocol)
}

func getExcludePortRanges(ctx context.Context, excludePort string, ignorePeerPorts bool, cl spec.Channel) ([][]int, error) {
	excludePortRanges, err := parseIntegerListToPortRanges("exclude-port", excludePort)
	if err != nil {
		return [][]int{}, err
	}
	// add peer ports
	portSet := make(map[int]interface{}, 0)
	for _, exexcludePortRange := range excludePortRanges {
		startPort := exexcludePortRange[0]
		endPort := exexcludePortRange[1]
		for p := startPort; p <= endPort; p++ {
			if _, ok := portSet[p]; !ok {
				portSet[p] = struct{}{}
			}
			if !ignorePeerPorts {
				peerPorts, err := getPeerPorts(ctx, strconv.Itoa(p), cl)
				if err != nil {
					log.Warnf(ctx, "get peer ports for %d err, %v", p, err)
					errMsg := fmt.Sprintf("get peer ports for %d err, %v, please solve the problem or skip to exclude peer ports by --ignore-peer-port flag", p, err)
					return nil, errors.New(errMsg)
				}
				log.Infof(ctx, "peer ports for %d: %v", p, peerPorts)
				for _, mp := range peerPorts {
					if _, ok := portSet[mp]; ok {
						continue
					}
					portSet[mp] = struct{}{}
				}
			}
		}
	}
	return portSetToPortRanges(portSet), nil
}

func buildExcludeFilterToNewBand(netInterface string, excludePortRanges [][]int, excludeIp string) string {
	var args string
	excludeIpRules := getExcludeIpRules(excludeIp)
	for _, rule := range excludeIpRules {
		args = fmt.Sprintf(
			`%s && \
			tc filter add dev %s parent 1: prio 1 protocol ip u32 %s flowid 1:4`,
			args, netInterface, rule)
	}

	for _, portRange := range excludePortRanges {
		masks := buildMaskForRange(portRange[0], portRange[1])
		for _, mask := range masks {
			args = fmt.Sprintf(
				`%s && \
                tc filter add dev %s parent 1: prio 1 protocol ip u32 match ip dport %d %#x flowid 1:4 && \
                tc filter add dev %s parent 1: prio 1 protocol ip u32 match ip sport %d %#x flowid 1:4`,
				args, netInterface, mask[0], mask[1], netInterface, mask[0], mask[1])
		}
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

func preHandleTxqueue(ctx context.Context, netInterface string, cl spec.Channel) *spec.Response {
	txFile := fmt.Sprintf("/sys/class/net/%s/tx_queue_len", netInterface)
	isExist := exec.CheckFilepathExists(ctx, cl, txFile)
	if isExist {
		// check the value
		response := cl.Run(ctx, "head", fmt.Sprintf("-1 %s", txFile))
		if response.Success {
			txlen := strings.TrimSpace(response.Result.(string))
			len, err := strconv.Atoi(txlen)
			if err != nil {
				log.Warnf(ctx, "parse %s file err, %v", txFile, err)
			} else {
				if len > 0 {
					return response
				} else {
					log.Infof(ctx, "the tx_queue_len value for %s is %s", netInterface, txlen)
				}
			}
		}
	}
	if cl.IsCommandAvailable(ctx, "ifconfig") {
		// set to 1000 directly
		response := cl.Run(ctx, "ifconfig", fmt.Sprintf("%s txqueuelen 1000", netInterface))
		if !response.Success {
			log.Warnf(ctx, "set txqueuelen for %s err, %s", netInterface, response.Err)
		}
	}
	return spec.ReturnSuccess("success")
}

func getIpRules(targetIp string) []string {
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

func getExcludeIpRules(excludeIp string) []string {
	if excludeIp == "" {
		return []string{}
	}
	ipString := strings.TrimSpace(excludeIp)
	ips := strings.Split(ipString, delimiter)
	ipRules := make([]string, 0)
	for _, ip := range ips {
		if strings.TrimSpace(ip) == "" {
			continue
		}
		// For exclude-ip, we need to match both source and destination IP
		ipRules = append(ipRules, fmt.Sprintf("match ip src %s", ip))
		ipRules = append(ipRules, fmt.Sprintf("match ip dst %s", ip))
	}
	return ipRules
}

// executeTargetPortAndIpWithExclude creates class rule in 1:4 queue and add filter to the queue
func executeTargetPortAndIpWithExclude(ctx context.Context, channel spec.Channel,
	netInterface, classRule string, localPortRanges, remotePortRanges [][]int, destIpRules []string, excludePorts [][]int, excludeIpRules []string, protocol string,
) *spec.Response {
	args := fmt.Sprintf(`qdisc add dev %s parent 1:4 handle 40: %s`, netInterface, classRule)
	args = buildTargetFilterPortAndIp(localPortRanges, remotePortRanges, destIpRules, excludePorts, excludeIpRules, args, netInterface, protocol)
	response := channel.Run(ctx, tcCommand(), args)
	if !response.Success {
		stopNet(ctx, netInterface, channel)
		return response
	}
	if verifyResp := verifyNetemApplied(ctx, channel, netInterface, classRule); !verifyResp.Success {
		stopNet(ctx, netInterface, channel)
		return verifyResp
	}
	return response
}

func buildTargetFilterPortAndIp(localPortRanges, remotePortRanges [][]int, destIpRules []string, excludePortRanges [][]int,
	excludeIpRules []string, args string, netInterface string, protocol string,
) string {
	protocolrule := ""
	if protocol != "" {
		if len(localPortRanges) == 0 && len(remotePortRanges) == 0 && len(destIpRules) == 0 && len(excludePortRanges) == 0 && len(excludeIpRules) == 0 {
			args = fmt.Sprintf(
				`%s && \
                tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip protocol %s 0xff flowid 1:4`,
				args, netInterface, protocol)
			return args
		} else {
			protocolrule = fmt.Sprintf(` \
                                         match ip protocol %s 0xff`, protocol)
		}
	}
	if len(localPortRanges) > 0 {
		for _, localPortRange := range localPortRanges {
			masks := buildMaskForRange(localPortRange[0], localPortRange[1])
			for _, mask := range masks {
				if len(destIpRules) > 0 {
					for _, ipRule := range destIpRules {
						args = fmt.Sprintf(
							`%s && \
                            tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip sport %d %#x %s flowid 1:4`,
							args, netInterface, ipRule, mask[0], mask[1], protocolrule)
					}
				} else {
					args = fmt.Sprintf(
						`%s && \
                        tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip sport %d %#x %s flowid 1:4`,
						args, netInterface, mask[0], mask[1], protocolrule)
				}
			}
		}
	}

	if len(remotePortRanges) > 0 {
		for _, remotePortRange := range remotePortRanges {
			masks := buildMaskForRange(remotePortRange[0], remotePortRange[1])
			for _, mask := range masks {
				if len(destIpRules) > 0 {
					for _, ipRule := range destIpRules {
						args = fmt.Sprintf(
							`%s && \
                            tc filter add dev %s parent 1: prio 4 protocol ip u32 %s match ip dport %d %#x %s flowid 1:4`,
							args, netInterface, ipRule, mask[0], mask[1], protocolrule)
					}
				} else {
					args = fmt.Sprintf(
						`%s && \
                        tc filter add dev %s parent 1: prio 4 protocol ip u32 match ip dport %d %#x %s flowid 1:4`,
						args, netInterface, mask[0], mask[1], protocolrule)
				}
			}
		}
	}
	if len(localPortRanges) == 0 && len(remotePortRanges) == 0 {
		// only destIp
		for _, ipRule := range destIpRules {
			args = fmt.Sprintf(
				`%s && \
				tc filter add dev %s parent 1: prio 4 protocol ip u32 %s %s flowid 1:4`,
				args, netInterface, ipRule, protocolrule)
		}
	}
	if len(excludeIpRules) > 0 {
		for _, ipRule := range excludeIpRules {
			args = fmt.Sprintf(
				`%s && \
				tc filter add dev %s parent 1: prio 1 protocol ip u32 %s %s flowid 1:3`,
				args, netInterface, ipRule, protocolrule)
		}
	}

	if len(excludePortRanges) > 0 {
		for _, excludePortRange := range excludePortRanges {
			masks := buildMaskForRange(excludePortRange[0], excludePortRange[1])
			for _, mask := range masks {
				args = fmt.Sprintf(
					`%s && \
                    tc filter add dev %s parent 1: prio 1 protocol ip u32 match ip dport %d %#x %s flowid 1:3 && \
                    tc filter add dev %s parent 1: prio 1 protocol ip u32 match ip sport %d %#x %s flowid 1:3`,
					args, netInterface, mask[0], mask[1], protocolrule, netInterface, mask[0], mask[1], protocolrule)
			}
		}
	}
	return args
}

// addQdiscForDL creates bands for filter
func addQdiscForDL(channel spec.Channel, ctx context.Context, netInterface string) *spec.Response {
	// add tc filter for delay specify port
	return channel.Run(ctx, tcCommand(), fmt.Sprintf(`qdisc add dev %s root handle 1: prio bands 4`, netInterface))
}

// stopNet
func stopNet(ctx context.Context, netInterface string, cl spec.Channel) *spec.Response {
	if os.Getuid() != 0 {
		return spec.ReturnFail(spec.Forbidden, fmt.Sprintf("tc no permission"))
	}
	response := cl.Run(ctx, tcCommand(), fmt.Sprintf(`filter show dev %s parent 1: prio 4`, netInterface))
	if response.Success && response.Result != "" {
		response = cl.Run(ctx, tcCommand(), fmt.Sprintf(`filter del dev %s parent 1: prio 4`, netInterface))
		if !response.Success {
			log.Errorf(ctx, "tc del filter err, %s", response.Err)
		}
	}
	return cl.Run(ctx, tcCommand(), fmt.Sprintf(`qdisc del dev %s root`, netInterface))
}

// getPeerPorts returns all ports communicating with the port
func getPeerPorts(ctx context.Context, port string, cl spec.Channel) ([]int, error) {
	if !cl.IsCommandAvailable(ctx, "ss") {
		return nil, errors.New(spec.CommandSsNotFound.Msg)
	}
	response := cl.Run(ctx, "ss", fmt.Sprintf("-n sport = %s or dport = %s", port, port))
	if !response.Success {
		return nil, errors.New(response.Err)
	}
	if util.IsNil(response.Result) {
		return []int{}, nil
	}
	result := response.Result.(string)
	ssMsg := strings.TrimSpace(result)
	if ssMsg == "" {
		return []int{}, nil
	}
	sockets := strings.Split(ssMsg, "\n")
	log.Infof(ctx, "sockets for %s, %v", port, sockets)
	mappingPorts := make([]int, 0)
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
				// for ipv6 address
				ipPort = strings.Split(f, "]:")
				if len(ipPort) != 2 {
					log.Warnf(ctx, "illegal socket address: %s", f)
					continue
				}
			}
			port, err := strconv.Atoi(ipPort[1])
			if err != nil {
				log.Warnf(ctx, "illegal port number: %s", f)
				continue
			}
			mappingPorts = append(mappingPorts, port)
		}
	}
	return mappingPorts, nil
}

func parseIntegerListToPortRanges(flagName string, flagValue string) ([][]int, error) {
	dedup := make(map[int]interface{})
	commaParts := strings.Split(flagValue, ",")
	for _, part := range commaParts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if !strings.Contains(value, "-") {
			intValue, err := strconv.Atoi(value)
			if err != nil {
				return nil, errors.New(spec.ParameterIllegal.Sprintf(flagName, flagValue, err))
			}
			dedup[intValue] = struct{}{}
			continue
		}
		ranges := strings.Split(value, "-")
		if len(ranges) != 2 {
			return nil, errors.New(spec.ParameterIllegal.Sprintf(flagName, flagValue,
				"Does not conform to the data format, a connector is required"))
		}
		startIndex, err := strconv.Atoi(strings.TrimSpace(ranges[0]))
		if err != nil {
			return nil, errors.New(spec.ParameterIllegal.Sprintf(flagName, flagValue, err))
		}
		endIndex, err := strconv.Atoi(strings.TrimSpace(ranges[1]))
		if err != nil {
			return nil, errors.New(spec.ParameterIllegal.Sprintf(flagName, flagValue, err))
		}
		if startIndex <= 0 || startIndex > math.MaxUint16 || endIndex <= 0 || endIndex >= math.MaxUint16 || endIndex < startIndex {
			return nil, errors.New(spec.ParameterIllegal.Sprintf(flagName, flagValue, "Illegal port range"))
		}
		for i := startIndex; i <= endIndex; i++ {
			dedup[i] = struct{}{}
		}
	}
	return portSetToPortRanges(dedup), nil
}

func portSetToPortRanges(portSet map[int]interface{}) [][]int {
	list := make([]int, 0)
	for k := range portSet {
		list = append(list, k)
	}
	sort.Ints(list)
	ranges := make([][]int, 0)
	for i := 0; i < len(list); {
		s := list[i]
		curRange := []int{s, s}
		j := i + 1
		for ; j < len(list) && list[j] == list[j-1]+1; j++ {
		}
		curRange[1] = list[j-1]
		i = j
		ranges = append(ranges, curRange)
	}
	return ranges
}

func buildMaskForRange(start, end int) [][]uint16 {
	cur := start
	masks := make([][]uint16, 0)
	for cur <= end {
		x := (1 << bits.Len(uint(cur))) - 1
		if end < x {
			x = end
		}
		o := 0
		for (cur & (1 << o)) == 0 {
			o++
		}
		mask := ^0
		cnt := 0
		for {
			upper := cur + (1 << cnt) - 1
			if cnt == o && upper <= x {
				break
			}
			if upper > x {
				mask = mask >> 1
				cnt--
				break
			} else {
				mask <<= 1
				cnt++
			}
		}
		masks = append(masks, []uint16{uint16(cur), uint16(mask)})
		cur = cur + (1 << cnt)
	}
	return masks
}
