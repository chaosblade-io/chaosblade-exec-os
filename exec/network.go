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

package exec

import (
	"fmt"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type NetworkCommandSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewNetworkCommandSpec() spec.ExpModelCommandSpec {
	return &NetworkCommandSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				NewDelayActionSpec(),
				NewDropActionSpec(),
				NewDnsActionSpec(),
				NewLossActionSpec(),
				NewDuplicateActionSpec(),
				NewCorruptActionSpec(),
				NewReorderActionSpec(),
				NewOccupyActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}

func (*NetworkCommandSpec) Name() string {
	return "network"
}

func (*NetworkCommandSpec) ShortDesc() string {
	return "Network experiment"
}

func (*NetworkCommandSpec) LongDesc() string {
	return "Network experiment"
}

func (*NetworkCommandSpec) Example() string {
	return `network delay --interface eth0 --time 3000

# You can execute "blade query network interface" command to query the interfaces`
}

// tcNetworkBin for network delay, loss, duplicate, reorder and corrupt experiments
var tcNetworkBin = "chaos_tcnetwork"

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
		Name:   "force",
		Desc:   "Forcibly overwrites the original rules",
		NoArgs: true,
	},
}

func getCommArgs(localPort, remotePort, excludePort, destinationIp, excludeIp string,
	args string, ignorePeerPort, force bool) (string, error) {
	if localPort != "" {
		localPorts, err := util.ParseIntegerListToStringSlice(localPort)
		if err != nil {
			return "", err
		}
		args = fmt.Sprintf("%s --local-port %s", args, strings.Join(localPorts, ","))
	}
	if remotePort != "" {
		remotePorts, err := util.ParseIntegerListToStringSlice(remotePort)
		if err != nil {
			return "", err
		}
		args = fmt.Sprintf("%s --remote-port %s", args, strings.Join(remotePorts, ","))
	}
	if excludePort != "" {
		excludePorts, err := util.ParseIntegerListToStringSlice(excludePort)
		if err != nil {
			return "", err
		}
		args = fmt.Sprintf("%s --exclude-port %s", args, strings.Join(excludePorts, ","))
	}
	if destinationIp != "" {
		args = fmt.Sprintf("%s --destination-ip %s", args, destinationIp)
	}
	if excludeIp != "" {
		args = fmt.Sprintf("%s --exclude-ip %s", args, excludeIp)
	}
	if ignorePeerPort {
		args = fmt.Sprintf("%s --ignore-peer-port", args)
	}
	if force {
		args = fmt.Sprintf("%s --force", args)
	}
	return args, nil
}

func checkNetworkExpEnv() error {
	commands := []string{"tc", "head", "ifconfig"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
