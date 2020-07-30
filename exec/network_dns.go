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
	"context"
	"fmt"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

type DnsActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewDnsActionSpec() spec.ExpActionCommandSpec {
	return &DnsActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:                  "domain",
					Desc:                  "Domain name",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
				&spec.ExpFlag{
					Name:                  "ip",
					Desc:                  "Domain ip",
					Required:              true,
					RequiredWhenDestroyed: true,
				},
			},
			ActionExecutor: &NetworkDnsExecutor{},
		},
	}
}

func (*DnsActionSpec) Name() string {
	return "dns"
}

func (*DnsActionSpec) Aliases() []string {
	return []string{}
}

func (*DnsActionSpec) ShortDesc() string {
	return "Dns experiment"
}

func (*DnsActionSpec) LongDesc() string {
	return "Dns experiment"
}

type NetworkDnsExecutor struct {
	channel spec.Channel
}

func (*NetworkDnsExecutor) Name() string {
	return "dns"
}

var changeDnsBin = "chaos_changedns"

func (ns *NetworkDnsExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	err := checkNetworkDnsExpEnv()
	if err != nil {
		return spec.ReturnFail(spec.Code[spec.CommandNotFound], err.Error())
	}
	if ns.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	domain := model.ActionFlags["domain"]
	ip := model.ActionFlags["ip"]
	if domain == "" || ip == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters],
			"less domain or ip arg for dns injection")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ns.stop(ctx, domain, ip)
	}

	return ns.start(ctx, domain, ip)
}

func (ns *NetworkDnsExecutor) start(ctx context.Context, domain, ip string) *spec.Response {
	return ns.channel.Run(ctx, path.Join(ns.channel.GetScriptPath(), changeDnsBin),
		fmt.Sprintf("--start --domain %s --ip %s --debug=%t", domain, ip, util.Debug))
}

func (ns *NetworkDnsExecutor) stop(ctx context.Context, domain, ip string) *spec.Response {
	return ns.channel.Run(ctx, path.Join(ns.channel.GetScriptPath(), changeDnsBin),
		fmt.Sprintf("--stop --domain %s --ip %s --debug=%t", domain, ip, util.Debug))
}

func (ns *NetworkDnsExecutor) SetChannel(channel spec.Channel) {
	ns.channel = channel
}

func checkNetworkDnsExpEnv() error {
	commands := []string{"grep", "cat", "rm", "echo"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
