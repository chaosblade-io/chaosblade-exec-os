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
	"net/http"
	osutil "os"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

var OccupyNetworkBin = "chaos_occupynetwork"

type OccupyActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewOccupyActionSpec() spec.ExpActionCommandSpec {
	return &OccupyActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "port",
					Desc:     "The port occupied",
					NoArgs:   false,
					Required: true,
				},
				&spec.ExpFlag{
					Name:     "force",
					Desc:     "Force kill the process which is using the port",
					NoArgs:   true,
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "cgroup-root",
					Desc:     "cgroup root path, default value /sys/fs/cgroup",
					NoArgs:   false,
					Required: false,
					Default:  "/sys/fs/cgroup",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &OccupyActionExecutor{},
			ActionExample: `
#Specify port 8080 occupancy
blade create network occupy --port 8080 --force

# The machine accesses external 14.215.177.39 machine (ping www.baidu.com) 80 port packet loss rate 100%
blade create network loss --percent 100 --interface eth0 --remote-port 80 --destination-ip 14.215.177.39`,
			ActionPrograms:    []string{OccupyNetworkBin},
			ActionCategories:  []string{category.SystemNetwork},
			ActionProcessHang: true,
		},
	}
}

func (*OccupyActionSpec) Name() string {
	return "occupy"
}

func (*OccupyActionSpec) Aliases() []string {
	return []string{}
}

func (*OccupyActionSpec) ShortDesc() string {
	return "Occupy the specify port"
}

func (o *OccupyActionSpec) LongDesc() string {
	if o.ActionLongDesc != "" {
		return o.ActionLongDesc
	}
	return "Occupy the specify port, if the port is used, it will return fail, except add --force flag"
}

type OccupyActionExecutor struct {
	channel spec.Channel
}

func (*OccupyActionExecutor) Name() string {
	return "occupy"
}

func (oae *OccupyActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	// check reboot permission
	if osutil.Geteuid() != 0 {
		// not root
		return spec.ResponseFailWithFlags(spec.Forbidden)
	}
	port := model.ActionFlags["port"]
	if port == "" {
		log.Errorf(ctx, "port is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "port")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return oae.stop(port, ctx)
	}
	force := model.ActionFlags["force"]
	if force == "true" {
		// search the process which is using the port and kill it
		// netstat -tuanp | awk '{print $4,$7}'| grep ":8182"|head -n 1
		response := oae.channel.Run(ctx, "netstat",
			fmt.Sprintf(`-tuanp | awk '{print $4,$7}' | grep ":%s" | head -n 1 | awk '{print $NF}'`, port))
		// 127.0.0.1:8182 2814/hblog
		if !response.Success {
			return response
		}
		processMsg := strings.TrimSpace(response.Result.(string))
		if processMsg != "" {
			idx := strings.Index(processMsg, "/")
			if idx > 0 {
				pid := processMsg[:idx]
				if pid != "" {
					response := oae.channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", pid))
					if !response.Success {
						return response
					}
				}
			}
		}
	}
	// start occupy process
	return oae.start(port, ctx)
}

func (oae *OccupyActionExecutor) start(port string, ctx context.Context) *spec.Response {
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("listen and serve fail %v", err))
	}
	return spec.Success()
}

func (oae *OccupyActionExecutor) stop(port string, ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx, "bin", OccupyNetworkBin)
	return exec.Destroy(ctx, oae.channel, "network occupy")
}

func (oae *OccupyActionExecutor) SetChannel(channel spec.Channel) {
	oae.channel = channel
}
