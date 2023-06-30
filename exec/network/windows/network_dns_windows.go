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
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network/tc"
)

const sep = ","

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

var changeDnsBin = "chaos_changedns"

func (ns *NetworkDnsExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	domain := model.ActionFlags["domain"]
	ip := model.ActionFlags["ip"]
	if domain == "" || ip == "" {
		log.Errorf(ctx, "domain|ip is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "domain|ip")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return ns.stop(ctx, domain, ip)
	} else if _, ok := spec.IsVerify(ctx); ok {
		return ns.verify(ctx, domain, ip)
	}

	return ns.start(ctx, domain, ip)
}

const hosts = "C:\\Windows\\System32\\drivers\\etc\\hosts"
const ipv4Reg = "((2(5[0-5]|[0-4]\\d))|[0-1]?\\d{1,2})(\\.((2(5[0-5]|[0-4]\\d))|[0-1]?\\d{1,2})){3}"

func (ns *NetworkDnsExecutor) start(ctx context.Context, domain, ip string) *spec.Response {
	domain = strings.ReplaceAll(domain, sep, " ")
	dnsPair := createDnsPair(domain, ip)
	response := exec.FileContainerContent(ctx, hosts, dnsPair)
	if response.Success {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s has been exist", dnsPair))
	}
	return ns.channel.Run(ctx, "echo", fmt.Sprintf(`%s >> %s`, dnsPair, hosts))
}

func (ns *NetworkDnsExecutor) stop(ctx context.Context, domain, ip string) *spec.Response {
	domain = strings.ReplaceAll(domain, sep, " ")
	dnsPair := createDnsPair(domain, ip)
	response := exec.FileContainerContent(ctx, hosts, dnsPair)
	if !response.Success {
		// nothing to do
		return spec.Success()
	}

	hostbytes, err := os.ReadFile(hosts)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.FileCantReadOrOpen, hosts)
	}
	originHost := strings.Replace(string(hostbytes), dnsPair, "", -1)
	err = os.WriteFile(hosts, []byte(originHost), 0666)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.FileCantReadOrOpen, hosts)
	}

	return spec.Success()
}

func (ns *NetworkDnsExecutor) verify(ctx context.Context, domain, ip string) *spec.Response {
	response := ns.channel.Run(ctx, "ping", domain)
	if !response.Success {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "ping", response.Err)
	}

	reg, err := regexp.Compile(ipv4Reg)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "regexp", err.Error())
	}
	solvedIp := string(reg.Find([]byte(response.Result.(string))))

	if ip != solvedIp {
		response = spec.ReturnFail(spec.DnsSelfVerifyFailed, solvedIp)
	} else {
		response = spec.ReturnSuccess(solvedIp)
	}
	response.Result = fmt.Sprintf("%s", solvedIp)
	return response
}

func (ns *NetworkDnsExecutor) SetChannel(channel spec.Channel) {
	ns.channel = channel
}

func createDnsPair(domain, ip string) string {
	return fmt.Sprintf("%s %s #chaosblade", ip, domain)
}
