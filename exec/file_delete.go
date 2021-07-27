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
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const DeleteFileBin = "chaos_deletefile"

type FileDeleteActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileDeleteActionSpec() spec.ExpActionCommandSpec {
	return &FileDeleteActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:   "force",
					Desc:   "use --force flag can't be restored",
					NoArgs: true,
				},
			},
			ActionExecutor: &FileRemoveActionExecutor{},
			ActionExample: `
# Delete the file /home/logs/nginx.log
blade create file delete --filepath /home/logs/nginx.log

# Force delete the file /home/logs/nginx.log unrecoverable
blade create file delete --filepath /home/logs/nginx.log --force
`,
			ActionPrograms:   []string{DeleteFileBin},
			ActionCategories: []string{category.SystemFile},
		},
	}
}

func (*FileDeleteActionSpec) Name() string {
	return "delete"
}

func (*FileDeleteActionSpec) Aliases() []string {
	return []string{}
}

func (*FileDeleteActionSpec) ShortDesc() string {
	return "File delete"
}

func (f *FileDeleteActionSpec) LongDesc() string {
	return "File delete"
}

type FileRemoveActionExecutor struct {
	channel spec.Channel
}

func (*FileRemoveActionExecutor) Name() string {
	return "remove"
}

func (f *FileRemoveActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"rm", "mv"}
	if response, ok := channel.NewLocalChannel().IsAllCommandsAvailable(commands); !ok {
		return response
	}

	if f.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}

	filepath := model.ActionFlags["filepath"]
	force := model.ActionFlags["force"] == "true"

	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, force, ctx)
	}

	if !util.IsExist(filepath) {
		util.Errorf(uid, util.GetRunFuncName(), fmt.Sprintf("`%s`: file does not exist", filepath))
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}

	return f.start(filepath, force, ctx)
}

func (f *FileRemoveActionExecutor) start(filepath string, force bool, ctx context.Context) *spec.Response {
	flags := fmt.Sprintf(`--start --filepath "%s" --debug=%t`, filepath, util.Debug)
	if force {
		flags = fmt.Sprintf(`%s --force=true`, flags)
	}
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), DeleteFileBin), flags)
}

func (f *FileRemoveActionExecutor) stop(filepath string, force bool, ctx context.Context) *spec.Response {
	filepath = strings.TrimPrefix(filepath, "'")
	filepath = strings.TrimSuffix(filepath, "'")
	flags := fmt.Sprintf(`--stop --filepath "%s" --debug=%t`, filepath, util.Debug)
	if force {
		flags = fmt.Sprintf(`%s --force=true`, flags)
	}
	return f.channel.Run(ctx, path.Join(f.channel.GetScriptPath(), DeleteFileBin), flags)
}

func (f *FileRemoveActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}
