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
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const BurnIOBin = "chaos_burnio"

type BurnActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewBurnActionSpec() spec.ExpActionCommandSpec {
	return &BurnActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:   "read",
					Desc:   "Burn io by read, it will create a 600M for reading and delete it when destroy it",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "write",
					Desc:   "Burn io by write, it will create a file by value of the size flag, for example the size default value is 10, then it will create a 10M*100=1000M file for writing, and delete it when destroy",
					NoArgs: true,
				},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "size",
					Desc: "Block size, MB, default is 10",
				},
				&spec.ExpFlag{
					Name: "path",
					Desc: "The path of directory where the disk is burning, default value is /",
				},
			},
			ActionExecutor: &BurnIOExecutor{},
			ActionExample: `
# The data of rkB/s, wkB/s and % Util were mainly observed. Perform disk read IO high-load scenarios
blade create disk burn --read --path /home

# Perform disk write IO high-load scenarios
blade create disk burn --write --path /home

# Read and write IO load scenarios are performed at the same time. Path is not specified. The default is /
blade create disk burn --read --write`,
			ActionPrograms:   []string{BurnIOBin},
			ActionCategories: []string{category.SystemDisk},
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
	commands := []string{"rm", "dd"}
	if response, ok := channel.NewLocalChannel().IsAllCommandsAvailable(commands); !ok {
		return response
	}
	if be.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	directory := "/"
	path := model.ActionFlags["path"]
	if path != "" {
		directory = path
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		readExists := model.ActionFlags["read"] == "true"
		writeExists := model.ActionFlags["write"] == "true"
		// set readExists and writeExists to true if does not specify read and write flags
		if !(readExists || writeExists) {
			readExists = true
			writeExists = true
		}
		return be.stop(ctx, readExists, writeExists, directory)
	}
	if !util.IsDir(directory) {
		util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: path is illegal, is not a directory", directory))
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "path", directory, "it must be a directory")
	}
	readExists := model.ActionFlags["read"] == "true"
	writeExists := model.ActionFlags["write"] == "true"
	if !readExists && !writeExists {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "read|write")
	}
	size := model.ActionFlags["size"]
	if size == "" {
		size = "10"
	}
	return be.start(ctx, readExists, writeExists, directory, size)
}

func (be *BurnIOExecutor) start(ctx context.Context, read, write bool, directory, size string) *spec.Response {
	return be.channel.Run(ctx, path.Join(be.channel.GetScriptPath(), BurnIOBin),
		fmt.Sprintf("--read=%t --write=%t --directory %s --size %s --start --debug=%t", read, write, directory, size, util.Debug))
}

func (be *BurnIOExecutor) stop(ctx context.Context, read, write bool, directory string) *spec.Response {
	return be.channel.Run(ctx, path.Join(be.channel.GetScriptPath(), BurnIOBin),
		fmt.Sprintf("--read=%t --write=%t --directory %s --stop --debug=%t", read, write, directory, util.Debug))
}

func (be *BurnIOExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}
