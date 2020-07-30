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

	"github.com/chaosblade-io/chaosblade-spec-go/channel"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var dnsDomain, dnsIp string
var changeDnsStart, changeDnsStop bool

func main() {
	flag.StringVar(&dnsDomain, "domain", "", "dns domain")
	flag.StringVar(&dnsIp, "ip", "", "dns ip")
	flag.BoolVar(&changeDnsStart, "start", false, "start change dns")
	flag.BoolVar(&changeDnsStop, "stop", false, "recover dns")
	bin.ParseFlagAndInitLog()

	if dnsDomain == "" || dnsIp == "" {
		bin.PrintErrAndExit("less --domain or --ip flag")
	}
	if changeDnsStart {
		startChangeDns(dnsDomain, dnsIp)
	} else if changeDnsStop {
		recoverDns(dnsDomain, dnsIp)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

const hosts = "/etc/hosts"
const tmpHosts = "/tmp/chaos-hosts.tmp"

var cl = channel.NewLocalChannel()

// startChangeDns by the domain and ip
func startChangeDns(domain, ip string) {
	ctx := context.Background()
	dnsPair := createDnsPair(domain, ip)
	response := cl.Run(ctx, "grep", fmt.Sprintf(`-q "%s" %s`, dnsPair, hosts))
	if response.Success {
		bin.PrintErrAndExit(fmt.Sprintf("%s has been exist", dnsPair))
		return
	}
	response = cl.Run(ctx, "echo", fmt.Sprintf(`"%s" >> %s`, dnsPair, hosts))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

// recoverDns
func recoverDns(domain, ip string) {
	ctx := context.Background()
	dnsPair := createDnsPair(domain, ip)
	response := cl.Run(ctx, "grep", fmt.Sprintf(`-q "%s" %s`, dnsPair, hosts))
	if !response.Success {
		bin.PrintOutputAndExit("nothing to do")
		return
	}
	response = cl.Run(ctx, "cat", fmt.Sprintf(`%s | grep -v "%s" > %s && cat %s > %s`,
		hosts, dnsPair, tmpHosts, tmpHosts, hosts))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	cl.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, tmpHosts))
}

func createDnsPair(domain, ip string) string {
	return fmt.Sprintf("%s %s #chaosblade", ip, domain)
}
