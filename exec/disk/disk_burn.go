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

package disk

import (
	"context"
	"fmt"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const BurnIOBin = "chaos_burnio"

type BurnActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewBurnActionSpec() spec.ExpActionCommandSpec {
	return &BurnActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&ReadFlag,
				&WriteFlag,
			},
			ActionFlags: []spec.ExpFlagSpec{
				&SizeFlag,
				&PathFlag,
			},
			ActionExecutor: &BurnIOExecutor{},
			ActionExample: `
# The data of rkB/s, wkB/s and % Util were mainly observed. Perform disk read IO high-load scenarios
blade create disk burn --read --path /home

# Perform disk write IO high-load scenarios
blade create disk burn --write --path /home

# Read and write IO load scenarios are performed at the same time. Path is not specified. The default is /
blade create disk burn --read --write`,
			ActionPrograms: []string{BurnIOBin},
		},
	}
}

func (*BurnActionSpec) Name() string {
	return "burn"
}

func (*BurnActionSpec) Aliases() []string {
	return []string{}
}
func (*BurnActionSpec) ShortDesc() string {
	return "Increase disk read and write io load"
}

func (b *BurnActionSpec) LongDesc() string {
	if b.ActionLongDesc != "" {
		return b.ActionLongDesc
	}
	return "Increase disk read and write io load"
}

type BurnIOExecutor struct {
	channel spec.Channel
}

func (*BurnIOExecutor) Name() string {
	return "burn"
}

func (be *BurnIOExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	matchers := spec.ConvertExpMatchersToString(model, func() map[string]spec.Empty { return make(map[string]spec.Empty, 0) })
	if _, ok := spec.IsDestroy(ctx); ok {
		return be.channel.Run(ctx, path.Join(be.channel.GetScriptPath(), BurnIOBin), fmt.Sprintf("--stop %s", matchers))
	}
	return be.channel.Run(ctx, path.Join(be.channel.GetScriptPath(), BurnIOBin), fmt.Sprintf("--start %s", matchers))
}

func (be *BurnIOExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}
