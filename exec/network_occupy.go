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
	osutil "os"
	"path"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

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
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &OccupyActionExecutor{},
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

func (*OccupyActionSpec) LongDesc() string {
	return "Occupy the specify port, if the port is used, it will return fail, except add --force flag"
}

type OccupyActionExecutor struct {
	channel spec.Channel
}

func (*OccupyActionExecutor) Name() string {
	return "occupy"
}

func (oae *OccupyActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if oae.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError], "channel is nil")
	}
	// check reboot permission
	if osutil.Geteuid() != 0 {
		// not root
		return spec.ReturnFail(spec.Code[spec.Forbidden], "must be root")
	}
	port := model.ActionFlags["port"]
	if port == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "less --port flag")
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

var OccupyNetworkBin = "chaos_occupynetwork"

func (oae *OccupyActionExecutor) start(port string, ctx context.Context) *spec.Response {
	return oae.channel.Run(ctx, path.Join(oae.channel.GetScriptPath(), OccupyNetworkBin),
		fmt.Sprintf("--start --port %s --debug=%t", port, util.Debug))
}

func (oae *OccupyActionExecutor) stop(port string, ctx context.Context) *spec.Response {
	return oae.channel.Run(ctx, path.Join(oae.channel.GetScriptPath(), OccupyNetworkBin),
		fmt.Sprintf("--stop --port %s --debug=%t", port, util.Debug))
}

func (oae *OccupyActionExecutor) SetChannel(channel spec.Channel) {
	oae.channel = channel
}
