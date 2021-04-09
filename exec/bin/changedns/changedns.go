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

package changedns

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(ChangeDNS))
}

type ChangeDNS struct {
	DNSDomain      string `name:"domain" json:"domain" yaml:"domain" default:"" help:"dns domain"`
	DNSIp          string `name:"ip" json:"ip" yaml:"ip" default:"" help:"dns ip"`
	ChangeDnsStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start change dns"`
	ChangeDnsStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"recover dns"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *ChangeDNS) Assign() model.Worker {
	worker := &ChangeDNS{Channel: channel.NewLocalChannel()}
	return worker
}

func (that *ChangeDNS) Name() string {
	return exec.ChangeDnsBin
}

func (that *ChangeDNS) Exec() *spec.Response {
	if that.DNSDomain == "" || that.DNSIp == "" {
		bin.PrintErrAndExit("less --domain or --ip flag")
	}
	if that.ChangeDnsStart {
		that.startChangeDns(that.DNSDomain, that.DNSIp)
	} else if that.ChangeDnsStop {
		that.recoverDns(that.DNSDomain, that.DNSIp)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

const hosts = "/etc/hosts"
const tmpHosts = "/tmp/chaos-hosts.tmp"

// startChangeDns by the domain and ip
func (that *ChangeDNS) startChangeDns(domain, ip string) {
	ctx := context.Background()
	dnsPair := createDnsPair(domain, ip)
	response := that.Channel.Run(ctx, "grep", fmt.Sprintf(`-q "%s" %s`, dnsPair, hosts))
	if response.Success {
		bin.PrintErrAndExit(fmt.Sprintf("%s has been exist", dnsPair))
		return
	}
	response = that.Channel.Run(ctx, "echo", fmt.Sprintf(`"%s" >> %s`, dnsPair, hosts))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

// recoverDns
func (that *ChangeDNS) recoverDns(domain, ip string) {
	ctx := context.Background()
	dnsPair := createDnsPair(domain, ip)
	response := that.Channel.Run(ctx, "grep", fmt.Sprintf(`-q "%s" %s`, dnsPair, hosts))
	if !response.Success {
		bin.PrintOutputAndExit("nothing to do")
		return
	}
	response = that.Channel.Run(ctx, "cat", fmt.Sprintf(`%s | grep -v "%s" > %s && cat %s > %s`,
		hosts, dnsPair, tmpHosts, tmpHosts, hosts))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	that.Channel.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, tmpHosts))
}

func createDnsPair(domain, ip string) string {
	return fmt.Sprintf("%s %s #chaosblade", ip, domain)
}
