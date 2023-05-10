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
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"strings"
)

type DnsDownActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func (*DnsDownActionSpec) Name() string {
	return "dns_down"
}

func (*DnsDownActionSpec) Aliases() []string {
	return []string{}
}

func (*DnsDownActionSpec) ShortDesc() string {
	return "Make DNS is not accessible"
}

func (d *DnsDownActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "Make DNS is not accessible."
}

const DnsDown = "chaos_dns_down"

func NewDnsDownActionSpec() spec.ExpActionCommandSpec {
	return &DnsDownActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:                  "allow_domain",
					Desc:                  "Domain names that resolve normally even when DNS is unavailable, with multiple domain names separated by ','. For example: test1.com,test2.com",
					Required:              false,
					RequiredWhenDestroyed: false,
				},
			},
			ActionExecutor: &NetworkDnsDownExecutor{},
			ActionExample: `
# The domain name www.baidu.com is not accessible while test1.com and test2.com are accessible.
blade create network dns_down --allow_domain test1.com,test2.com`,
			ActionPrograms:   []string{DnsDown},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

type NetworkDnsDownExecutor struct {
	channel spec.Channel
}

func (*NetworkDnsDownExecutor) Name() string {
	return "dns_down"
}

func (ns *NetworkDnsDownExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"grep", "cat", "rm", "echo", "iptables", "iptables-save", "iptables-restore", "nslookup", "awk", "ping", "tee", "tail"}
	if response, ok := ns.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}
	allowDomains := strings.Split(model.ActionFlags["allow_domain"], `,`)
	if _, ok := spec.IsDestroy(ctx); ok {
		return ns.stop(ctx, allowDomains)
	}
	return ns.start(ctx, allowDomains)
}

const iptablesBackup = "/tmp/iptables-backup.txt"

func (ns *NetworkDnsDownExecutor) start(ctx context.Context, allowDomains []string) *spec.Response {
	if len(allowDomains) > 0 {
		backHosts := ns.channel.Run(ctx, "cat", fmt.Sprintf("%s > %s", hosts, tmpHosts))
		if !backHosts.Success {
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("Backup hosts file failed. Error: %s", backHosts.Err))
		}
		for _, domain := range allowDomains {
			resp := ns.channel.Run(ctx, "", fmt.Sprintf(`domain="%s" && if ping -c 1 $domain > /dev/null; then echo "$(nslookup $domain | grep 'Address:' | grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}' | grep -v ':' | tail -1) $domain" | sudo tee -a %s; fi`, domain, hosts))
			if !resp.Success {
				_ = ns.channel.Run(ctx, "cat", fmt.Sprintf("%s > %s", tmpHosts, hosts)) // recover
				return spec.ReturnFail(spec.OsCmdExecFailed, resp.Err)
			}
		}
	}
	// backup iptables rules
	bkIptables := ns.channel.Run(ctx, "iptables-save", fmt.Sprintf("> %s", iptablesBackup))
	if !bkIptables.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, bkIptables.Error())
	}
	// dns_down
	dnsDown := ns.channel.Run(ctx, "iptables", "-A OUTPUT -p udp --dport 53 -j DROP; iptables -A OUTPUT -p tcp --dport 53 -j DROP")
	if !dnsDown.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf(`DNS dwon fialed for %s, you can use "iptables-restore < %s" to restore your iptable rules if needed.`, dnsDown.Err, iptablesBackup))
	}
	return dnsDown
}

func (ns *NetworkDnsDownExecutor) stop(ctx context.Context, allowDomains []string) *spec.Response {
	recoverDns := ns.channel.Run(ctx, "iptables-restore", fmt.Sprintf("< %s", iptablesBackup))
	if !recoverDns.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf(`DNS recover fialed for %s, you can use "iptables-restore < %s" to restore your iptables rules if needed.`, recoverDns.Err, iptablesBackup))
	}
	if len(allowDomains) > 0 {
		recoverHosts := ns.channel.Run(ctx, "cat", fmt.Sprintf("%s > %s", tmpHosts, hosts))
		if !recoverHosts.Success {
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("Restore hosts file failed. Error: %s, a backup of hosts is in %s", recoverHosts.Err, tmpHosts))
		}
	}
	return ns.channel.Run(ctx, "rm", fmt.Sprintf("-rf %s %s", tmpHosts, iptablesBackup))
}

func (ns *NetworkDnsDownExecutor) SetChannel(channel spec.Channel) {
	ns.channel = channel
}
