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

package systemd

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const StopSystemdBin = "chaos_stopsystemd"

type StopSystemdActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewStopSystemdActionCommandSpec() spec.ExpActionCommandSpec {
	return &StopSystemdActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "service",
					Desc: "Service name",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &StopSystemdExecutor{},
			ActionExample: `
 # Stop the service test
 blade create systemd stop --service test`,
			ActionPrograms:   []string{StopSystemdBin},
			ActionCategories: []string{category.SystemSystemd},
		},
	}
}

func (*StopSystemdActionCommandSpec) Name() string {
	return "stop"
}

func (*StopSystemdActionCommandSpec) Aliases() []string {
	return []string{"s"}
}

func (*StopSystemdActionCommandSpec) ShortDesc() string {
	return "Stop systemd"
}

func (k *StopSystemdActionCommandSpec) LongDesc() string {
	if k.ActionLongDesc != "" {
		return k.ActionLongDesc
	}
	return "Stop system by service name"
}

func (*StopSystemdActionCommandSpec) Categories() []string {
	return []string{category.SystemSystemd}
}

type StopSystemdExecutor struct {
	channel spec.Channel
}

func (sse *StopSystemdExecutor) Name() string {
	return "stop"
}

func (sse *StopSystemdExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	service := model.ActionFlags["service"]
	if service == "" {
		log.Errorf(ctx, "less service name")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "service")
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return sse.startService(service, ctx)
	} else {
		if response := checkServiceInvalid(uid, service, ctx, sse.channel); response != nil {
			return response
		}
		return sse.channel.Run(ctx, "systemctl", fmt.Sprintf("stop %s", service))
	}
}

func checkServiceInvalid(uid, service string, ctx context.Context, cl spec.Channel) *spec.Response {
	if !cl.IsCommandAvailable(ctx, "systemctl") {
		log.Errorf(ctx, spec.CommandSystemctlNotFound.Msg)
		return spec.ResponseFailWithFlags(spec.CommandSystemctlNotFound)
	}
	response := cl.Run(ctx, "systemctl", fmt.Sprintf(`status "%s" | grep 'Active' | grep 'running'`, service))
	if !response.Success {
		log.Errorf(ctx, spec.SystemdNotFound.Sprintf("service", response.Err))
		return spec.ResponseFailWithFlags(spec.SystemdNotFound, service, response.Err)
	}
	return nil
}

func (sse *StopSystemdExecutor) startService(service string, ctx context.Context) *spec.Response {
	return sse.channel.Run(ctx, "systemctl", fmt.Sprintf("start %s", service))
}

func (sse *StopSystemdExecutor) SetChannel(channel spec.Channel) {
	sse.channel = channel
}
