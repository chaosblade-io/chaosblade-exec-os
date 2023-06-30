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
	"net/http"
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
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &OccupyActionExecutor{},
			ActionExample: `
#Specify port 8080 occupancy
blade c network occupy --port 8080 --force`,
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
	//if osutil.Geteuid() != 0 {
	//	// not root
	//	return spec.ResponseFailWithFlags(spec.Forbidden)
	//}
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
		response := oae.KillProcessByPort(ctx, port)
		if !response.Success {
			return response
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

func (oae *OccupyActionExecutor) KillProcessByPort(ctx context.Context, port string) *spec.Response {
	// search the process which is using the port and kill it
	// netstat -aon|findstr ":8081"
	response := oae.channel.Run(ctx, "netstat",
		fmt.Sprintf(`-aon | findstr ":%s"`, port))
	// 127.0.0.1:8182 2814/hblog
	if !response.Success {
		return response
	}
	processMsg := strings.TrimSpace(response.Result.(string))
	if processMsg == "" {
		return spec.Success()
	}

	/**
	TCP    0.0.0.0:8081           0.0.0.0:0              LISTENING       4808
	TCP    30.000.76.137:58903    42.000.74.58:8081      ESTABLISHED     13764
	TCP    [::]:8081              [::]:0                 LISTENING       4808

	can use this to get port and pid
	for /f "tokens=2,5 delims= " %a in ('netstat -aon') do @echo %a %b | findstr 8081

	*/
	processinfos := strings.Split(processMsg, "\n")
	for _, processinfo := range processinfos {
		infos := strings.Fields(strings.TrimSpace(processinfo))
		if len(infos) < 5 {
			continue
		}
		if !strings.Contains(infos[1], fmt.Sprintf(":%s", port)) {
			continue
		}

		pid := strings.TrimSpace(infos[4])
		if pid == "" {
			continue
		}
		response := oae.channel.Kill(ctx, []string{pid})
		if !response.Success {
			return response
		}
	}

	return spec.Success()
}
