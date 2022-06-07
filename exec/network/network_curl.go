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
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const CurlNetworkBin = "chaos_curlnetwork"

type CurlActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewCurlActionSpec() spec.ExpActionCommandSpec {
	return &CurlActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "url",
					Desc:     "Url",
					Required: true,
				},
			},
			ActionExecutor: &NetworkCurlExecutor{},
			ActionExample: `
# Curl url
blade verify network curl --url 11.11.11.11:9001/result`,
			ActionPrograms:   []string{CurlNetworkBin},
			ActionCategories: []string{category.SystemNetwork},
		},
	}
}

func (*CurlActionSpec) Name() string {
	return "curl"
}

func (*CurlActionSpec) Aliases() []string {
	return []string{}
}

func (*CurlActionSpec) ShortDesc() string {
	return "Curl verify command"
}

func (c *CurlActionSpec) LongDesc() string {
	if c.ActionLongDesc != "" {
		return c.ActionLongDesc
	}
	return "Curl experiment"
}

type NetworkCurlExecutor struct {
	channel spec.Channel
}

func (*NetworkCurlExecutor) Name() string {
	return "curl"
}

func (nce *NetworkCurlExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"curl"}
	if response, ok := nce.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	url := model.ActionFlags["url"]
	if url == "" {
		log.Errorf(ctx, "url is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "url")
	}
	return nce.verify(ctx, url)
}

func (nce *NetworkCurlExecutor) verify(ctx context.Context, url string) *spec.Response {
	response := nce.channel.Run(ctx, "curl", fmt.Sprintf(`-s %s`, url))
	if !response.Success {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "curl", response.Err)
	}
	return spec.ReturnSuccess(response.Result)
}

func (nce *NetworkCurlExecutor) SetChannel(channel spec.Channel) {
	nce.channel = channel
}
