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

package file

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const ChmodFileBin = "chaos_chmodfile"

type FileChmodActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileChmodActionSpec() spec.ExpActionCommandSpec {
	return &FileChmodActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "mark",
					Desc:     "--mark 777",
					Required: true,
				},
			},
			ActionExecutor: &FileChmodActionExecutor{},
			ActionExample: `
# Modify /home/logs/nginx.log file permissions to 777
blade create file chmod --filepath /home/logs/nginx.log --mark=777
`,
			ActionPrograms:   []string{ChmodFileBin},
			ActionCategories: []string{category.SystemFile},
		},
	}
}

func (*FileChmodActionSpec) Name() string {
	return "chmod"
}

func (*FileChmodActionSpec) Aliases() []string {
	return []string{}
}

func (*FileChmodActionSpec) ShortDesc() string {
	return "File permission modification."
}

func (f *FileChmodActionSpec) LongDesc() string {
	return "File perçmission modification."
}

type FileChmodActionExecutor struct {
	channel spec.Channel
}

func (*FileChmodActionExecutor) Name() string {
	return "chmod"
}

const tmpFileChmod = "/tmp/chaos-file-chmod.tmp"

func (f *FileChmodActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"chmod", "grep", "echo", "rm", "awk", "cat", "stat"}
	if response, ok := f.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}
	mark := model.ActionFlags["mark"]
	match, _ := regexp.MatchString("^([0-7]{3})$", mark)
	if !match {
		log.Errorf(ctx, "`%s` mark is illegal", mark)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mark", mark, "the mark is not matched")
	}

	filepath := model.ActionFlags["filepath"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stopChmodFile(ctx, filepath, mark)
	}

	if !exec.CheckFilepathExists(ctx, f.channel, filepath) {
		log.Errorf(ctx, "`%s`: file does not exist", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}

	response := f.channel.Run(ctx, "grep", fmt.Sprintf(`-q "%s:" "%s"`, filepath, tmpFileChmod))
	if response.Success {
		log.Errorf(ctx, "%s is already being experimented", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "filepath", filepath, "already being experimented")
	}
	response = f.channel.Run(ctx, "stat", fmt.Sprintf(`-c "%%a" %s`, filepath))
	if !response.Success {
		log.Errorf(ctx, "`%s`: can't get file's origin mark", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "filepath", filepath, "can't get file's mark")
	}
	originMark := response.Result.(string)

	response = f.channel.Run(ctx, "echo", fmt.Sprintf(`'%s:%s' >> %s`, filepath, originMark, tmpFileChmod))
	if !response.Success {
		return response
	}
	return f.channel.Run(ctx, "chmod", fmt.Sprintf(`%s "%s"`, mark, filepath))
}

func (f *FileChmodActionExecutor) stopChmodFile(ctx context.Context, filepath, mark string) *spec.Response {
	// get origin mark
	response := f.channel.Run(ctx, "grep", fmt.Sprintf(`%s: %s | awk -F ':' '{printf $2}'`, filepath, tmpFileChmod))
	if !response.Success {
		f.clearTempFile(filepath, response, ctx)
		return response
	}
	originMark := response.Result.(string)
	response = f.channel.Run(ctx, "chmod", fmt.Sprintf(`%s %s`, originMark, filepath))
	f.clearTempFile(filepath, response, ctx)
	return response
}

func (f *FileChmodActionExecutor) clearTempFile(filepath string, response *spec.Response, ctx context.Context) {
	response = f.channel.Run(ctx, "cat", fmt.Sprintf(`"%s"| grep -v %s:`, tmpFileChmod, filepath))
	if !response.Success {
		response = f.channel.Run(ctx, "rm", fmt.Sprintf(`-rf "%s"`, tmpFileChmod))
		if !response.Success {
			log.Errorf(ctx, "clean temp file error %s", response.Err)
		}
	} else {
		response = f.channel.Run(ctx, "echo", fmt.Sprintf(`"%s" > %s`,
			strings.TrimRight(response.Result.(string), "\n"),
			tmpFileChmod))
		if !response.Success {
			log.Errorf(ctx, "clean temp file error %s", response.Err)
		}
	}
}

func (f *FileChmodActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}
