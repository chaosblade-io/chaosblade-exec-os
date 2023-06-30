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
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
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
					Desc:     "only for read-only or read-write",
					Required: true,
				},
			},
			ActionExecutor: &FileChmodActionExecutor{},
			ActionExample: `
# Modify C:\\nginx.log file permissions to 777
blade create file chmod --filepath C:\\nginx.log --mark=read-only
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

const READ_ONLY = "read-only"
const READ_WRITE = "read-write"

var tmpFileChmod = path.Join(util.GetProgramPath(), "chaos-file-chmod.txt")

func (f *FileChmodActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	mark := model.ActionFlags["mark"]
	//if mark == "" {
	//	log.Errorf(ctx, "`%s` mark is illegal", mark)
	//	return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mark", mark, "the mark is not matched")
	//}

	filepath := model.ActionFlags["filepath"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stopChmodFile(ctx, filepath, mark)
	}

	if !exec.CheckFilepathExists(ctx, f.channel, filepath) {
		log.Errorf(ctx, "`%s`: file does not exist", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}

	result, err := exec.FileExistContent(ctx, tmpFileChmod, fmt.Sprintf("%s:", filepath))
	if err != nil {
		return spec.ResponseFailWithFlags(spec.FileCantReadOrOpen, tmpFileChmod)
	}
	if result != "" {
		log.Errorf(ctx, "%s is already being experimented", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "filepath", filepath, "already being experimented")
	}

	fileInfo, _ := os.Stat(filepath)
	originMark := strconv.FormatInt(int64(fileInfo.Mode().Perm()), 8)
	response := f.channel.Run(ctx, "echo", fmt.Sprintf(`%s:0%s >> %s`, filepath, originMark, tmpFileChmod))
	if !response.Success {
		return response
	}
	switch strings.ToLower(mark) {
	case READ_ONLY:
		err = os.Chmod(filepath, 0400)
	case READ_WRITE:
		err = os.Chmod(filepath, 0666)
	default:
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "mark", mark, "the mark is not matched")
	}
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "chmod", err.Error())
	}
	return spec.Success()
}

func (f *FileChmodActionExecutor) stopChmodFile(ctx context.Context, filepath, mark string) *spec.Response {
	// get origin mark
	originMark, err := exec.FileExistContent(ctx, tmpFileChmod, fmt.Sprintf("%s:", filepath))
	if err != nil {
		// todo
		f.clearTempFile(filepath, ctx)
		return spec.ResponseFailWithFlags(spec.FileCantReadOrOpen, tmpFileChmod)
	}
	if originMark == "" {
		f.clearTempFile(filepath, ctx)
		return spec.Success()
	}
	perm, err := strconv.ParseUint(originMark, 0, 32)
	if err != nil || perm&uint64(fs.ModePerm) != perm {
		f.clearTempFile(filepath, ctx)
		log.Errorf(ctx, "invalid mode: %s", originMark)
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "get origin mark", err.Error())
	}

	err = os.Chmod(filepath, os.FileMode(perm))
	f.clearTempFile(filepath, ctx)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "chmod", err.Error())
	}
	return spec.Success()
}

func (f *FileChmodActionExecutor) clearTempFile(filepath string, ctx context.Context) {
	buf, err := os.ReadFile(tmpFileChmod)
	if err != nil {
		log.Errorf(ctx, "read `%s` failed, err: %s", tmpFileChmod, err.Error())
		return
	}

	bufs := strings.Split(string(buf), "\n")
	var result string
	for _, info := range bufs {
		if !strings.Contains(info, fmt.Sprintf("%s:", filepath)) {
			continue
		}
		result = strings.Replace(string(buf), fmt.Sprintf("%s\n", info), "", 1)
		break
	}
	result = strings.Trim(result, "\n")

	if result == "" {
		err = os.Remove(tmpFileChmod)
	} else {
		err = os.WriteFile(tmpFileChmod, []byte(result), 0666)
	}

	if err != nil {
		log.Errorf(ctx, "clean temp file error %s", err.Error())
	}
}

func (f *FileChmodActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}
