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
	"os"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/goodhosts/hostsfile"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network/tc"
)

var hosts = hostsfile.HostsFilePath

const (
	tmpHosts = "/tmp/chaos-hosts.tmp"
	sep      = ","
	// backupHostsFileFormat hostsfilePath.back-$uid
	backupHostsFileFormat = "%s-%s"
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
				&spec.ExpFlag{
					Name: "replace",
					Desc: "If the domain name provided by the user conflicts with the original " +
						"domain name, whether to replace the original domain name, it will not be replaced by default.",
					NoArgs:   true,
					Required: false,
					Default:  "false",
				},
			},
			ActionExecutor: &NetworkDnsExecutor{},
			ActionExample: `
# The domain name www.baidu.com is not accessible
blade create network dns --domain www.baidu.com --ip 10.0.0.0`,
			ActionPrograms:   []string{tc.TcNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
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

func (d *DnsActionSpec) LongDesc() string {
	if d.ActionLongDesc != "" {
		return d.ActionLongDesc
	}
	return "Dns experiment"
}

type NetworkDnsExecutor struct {
	channel spec.Channel
}

func (*NetworkDnsExecutor) Name() string {
	return "dns"
}

func (ns *NetworkDnsExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"grep", "cat", "rm", "echo"}
	if response, ok := ns.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	domain := model.ActionFlags["domain"]
	ip := model.ActionFlags["ip"]
	if domain == "" || ip == "" {
		log.Errorf(ctx, "domain|ip is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "domain|ip")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ns.stop(ctx, uid)
	}

	var (
		replace bool
		err     error
	)
	if val := model.ActionFlags["replace"]; len(val) > 0 {
		if replace, err = strconv.ParseBool(strings.ToLower(model.ActionFlags["replace"])); err != nil {
			log.Warnf(ctx, "parse replace flag err: %v", err)
		}
	}

	// backup hosts file for recover
	if resp := ns.backupHostFile(ctx, uid); resp != nil && !resp.Success {
		log.Errorf(ctx, "read hosts file failed, err: %v, uid: %s", resp.Error(), uid)
		return resp
	}

	applier := newDnsApplier(ns.channel, replace)
	return applier.Start(ctx, uid, domain, ip)
}

func (ns *NetworkDnsExecutor) stop(ctx context.Context, uid string) *spec.Response {
	expHostsFile := fmt.Sprintf(backupHostsFileFormat, hosts, uid)
	response := ns.channel.Run(ctx, "cat", fmt.Sprintf("%s > %s", expHostsFile, hosts))
	if !response.Success {
		if strings.Contains(response.Err, "No such file or directory") {
			log.Warnf(ctx, "can not find backup hosts file for uid: %s", uid)
			return spec.ReturnSuccess("The hosts file has been recovered")
		}
		log.Errorf(ctx, "recover hosts file failed, %v, uid: %s", response.Err, uid)
		return response
	}
	log.Infof(ctx, "recover hosts file successfully, pre: %s", expHostsFile)
	return response
}

func (ns *NetworkDnsExecutor) SetChannel(channel spec.Channel) {
	ns.channel = channel
}

func createDnsPair(domain, ip string) string {
	return fmt.Sprintf("%s %s #chaosblade", ip, domain)
}

func (ns *NetworkDnsExecutor) backupHostFile(ctx context.Context, uid string) *spec.Response {
	response := ns.channel.Run(ctx, "cp", fmt.Sprintf(
		"%s %s", hosts, fmt.Sprintf(backupHostsFileFormat, hosts, uid),
	))
	if !response.Success {
		return response
	}
	return response
}

type dnsApplier interface {
	Start(ctx context.Context, uid, domainArg, ip string) *spec.Response
}

type defaultApplier struct{ ch spec.Channel }

type replaceApplier struct{ ch spec.Channel }

func newDnsApplier(ch spec.Channel, replace bool) dnsApplier {
	if replace {
		return &replaceApplier{ch: ch}
	}
	return &defaultApplier{ch: ch}
}

func (m *defaultApplier) Start(ctx context.Context, _, domainArg, ip string) *spec.Response {
	domainArg = strings.ReplaceAll(domainArg, sep, " ")
	dnsPair := createDnsPair(domainArg, ip)
	resp := m.ch.Run(ctx, "grep", fmt.Sprintf(`-q "%s" %s`, dnsPair, hosts))
	if resp.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s has been exist", dnsPair))
	}
	return m.ch.Run(ctx, "echo", fmt.Sprintf(`"%s" >> %s`, dnsPair, hosts))
}

func (m *replaceApplier) Start(ctx context.Context, uid, domainArg, ip string) *spec.Response {
	domains := make([]string, 0)
	for _, v := range strings.Split(domainArg, sep) {
		if d := strings.TrimSpace(v); len(d) > 0 {
			domains = append(domains, d)
		}
	}

	expHostsFile := fmt.Sprintf(backupHostsFileFormat, hosts, uid)
	// cat /etc/hosts
	response := m.ch.Run(ctx, "cat", hosts)
	if !response.Success {
		log.Errorf(ctx, "read hosts file failed, %v, uid: %s", response.Err, uid)
		return response
	}
	log.Debugf(ctx, "read and copy hosts file successfully, uid: %s, backup: %s", uid, expHostsFile)
	content, ok := response.Result.(string)
	if ok {
		log.Debugf(ctx, "read hosts file successfully, uid: %s, content: %s", uid, content)
	} else {
		log.Errorf(ctx, "read target's hosts file failed, uid: %s", uid)
		return response
	}

	temp, err := os.CreateTemp("", "chaosblade-dns-")
	if err != nil {
		log.Errorf(ctx, "create temp file failed, %v, uid: %s", err, uid)
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "create temp file failed for dns infusion")
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(temp.Name())

	if err := os.WriteFile(temp.Name(), []byte(content), 0o644); err != nil {
		log.Errorf(ctx, "write temp file failed, %v, uid: %s", err, uid)
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "write temp file failed for dns infusion")
	}

	customHosts, err := hostsfile.NewCustomHosts(temp.Name())
	if err != nil {
		log.Errorf(ctx, "create hostsfile entity failed, %v, uid: %s", err, uid)
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "create hostsfile entity failed for dns infusion")
	}

	if err := customHosts.Add(ip, domains...); err != nil {
		log.Errorf(ctx, "add dns pair failed, %v, uid: %s", err, uid)
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "add dns pair failed for dns infusion")
	}
	if err := customHosts.Flush(); err != nil {
		log.Errorf(ctx, "flush hosts file failed, %v, uid: %s", err, uid)
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "flush hosts file failed for dns infusion")
	}
	log.Debugf(ctx, "add dns pair successfully, uid: %s", uid)

	// echo "$(hostsfile content)" > /etc/hosts
	response = m.ch.Run(ctx, "echo", fmt.Sprintf("\"%s\" > %s", customHosts.String(), hosts))
	if !response.Success {
		log.Errorf(ctx, "write hosts file failed, %v, uid: %s", response.Err, uid)
		return response
	}
	log.Infof(ctx, "write hosts file successfully, uid: %s, backup: %s", uid, expHostsFile)
	return response
}
