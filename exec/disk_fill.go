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
	"strconv"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const FillDiskBin = "chaos_filldisk"

type FillActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFillActionSpec() spec.ExpActionCommandSpec {
	return &FillActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "path",
					Desc: "The path of directory where the disk is populated, default value is /",
				},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "size",
					Desc: "Disk fill size, unit is MB. The value is a positive integer without unit, for example, --size 1024",
				},
				&spec.ExpFlag{
					Name: "percent",
					Desc: "Total percentage of disk occupied by the specified path. If size and the flag exist, use this flag first. The value must be positive integer without %",
				},
				&spec.ExpFlag{
					Name: "reserve",
					Desc: "Disk reserve size, unit is MB. The value is a positive integer without unit. If size, percent and reserve flags exist, the priority is as follows: percent > reserve > size",
				},
				&spec.ExpFlag{
					Name:   "retain-handle",
					Desc:   "Whether to retain the big file handle, default value is false.",
					NoArgs: true,
				},
			},
			ActionExecutor: &FillActionExecutor{},
			ActionExample: `
# Perform a disk fill of 40G to achieve a full disk (34G available)
blade create disk fill --path /home --size 40000

# Performs populating the disk by percentage, and retains the file handle that populates the disk
Command: "blade c disk fill --path /home --percent 80 --retain-handle

# Perform a fixed-size experimental scenario
blade c disk fill --path /home --reserve 1024`,
			ActionPrograms:   []string{FillDiskBin},
			ActionCategories: []string{category.SystemDisk},
		},
	}
}

func (*FillActionSpec) Name() string {
	return "fill"
}

func (*FillActionSpec) Aliases() []string {
	return []string{}
}

func (*FillActionSpec) ShortDesc() string {
	return "Fill the specified directory path"
}

func (f *FillActionSpec) LongDesc() string {
	if f.ActionLongDesc != "" {
		return f.ActionLongDesc
	}
	return "Fill the specified directory path. If the path is not directory or does not exist, an error message will be returned."
}

type FillActionExecutor struct {
	channel spec.Channel
}

func (*FillActionExecutor) Name() string {
	return "fill"
}

func (fae *FillActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if fae.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	directory := "/"
	path := model.ActionFlags["path"]
	if path != "" {
		directory = path
	}
	if !util.IsDir(directory) {
		util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: path is illegal, is not a directory", directory))
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "path", directory, "it must be a directory")
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return fae.stop(directory, ctx)
	} else {
		retainHandle := model.ActionFlags["retain-handle"] == "true"
		percent := model.ActionFlags["percent"]
		if percent == "" {
			reserve := model.ActionFlags["reserve"]
			if reserve == "" {
				size := model.ActionFlags["size"]
				if size == "" {
					return spec.ResponseFailWithFlags(spec.ParameterLess, "size|percent")
				}
				_, err := strconv.Atoi(size)
				if err != nil {
					util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: size is illegal, it must be positive integer", size))
					return spec.ResponseFailWithFlags(spec.ParameterIllegal, "size", size, "it must be positive integer")
				}
				return fae.start(directory, size, percent, reserve, retainHandle, ctx)
			}
			_, err := strconv.Atoi(reserve)
			if err != nil {
				util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: reserve is illegal, it must be positive integer", reserve))
				return spec.ResponseFailWithFlags(spec.ParameterIllegal, "reserve", reserve, "it must be positive integer")
			}
			return fae.start(directory, "", percent, reserve, retainHandle, ctx)
		}
		_, err := strconv.Atoi(percent)
		if err != nil {
			util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: percent is illegal, it must be positive integer", percent))
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "percent", percent, "it must be positive integer")
		}
		return fae.start(directory, "", percent, "", retainHandle, ctx)
	}
}

func (fae *FillActionExecutor) start(directory, size, percent, reserve string, retainHandle bool, ctx context.Context) *spec.Response {
	flags := fmt.Sprintf("--directory %s --start --debug=%t --retain-handle=%t", directory, util.Debug, retainHandle)
	if percent != "" {
		flags = fmt.Sprintf("%s --percent %s", flags, percent)
	} else if reserve != "" {
		flags = fmt.Sprintf("%s --reserve %s", flags, reserve)
	} else {
		flags = fmt.Sprintf("%s --size %s", flags, size)
	}
	return fae.channel.Run(ctx, path.Join(fae.channel.GetScriptPath(), FillDiskBin), flags)
}

func (fae *FillActionExecutor) stop(directory string, ctx context.Context) *spec.Response {
	return fae.channel.Run(ctx, path.Join(fae.channel.GetScriptPath(), FillDiskBin),
		fmt.Sprintf("--directory %s --stop --debug=%t", directory, util.Debug))
}

func (fae *FillActionExecutor) SetChannel(channel spec.Channel) {
	fae.channel = channel
}
