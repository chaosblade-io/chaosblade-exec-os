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
	"os"
	osExec "os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const FileLoadBin = "chaos_fileload"

const DefaultFilePath = "chaos_load_file"

type FileLoadActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileLoadActionSpec() spec.ExpActionCommandSpec {
	return &FileLoadActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "count",
					Desc: "the number of append count, 0 or not set means unlimited",
				},
				&spec.ExpFlag{
					Name:   "force",
					Desc:   "use --force flag mean the experiment cannot automatically recover due to exceeding the file handle limit.",
					NoArgs: true,
				},
			},
			ActionExecutor: &FileLoadExecutor{},
			ActionExample: `
# open /home/logs/nginx.log 10 times
blade c file load --filepath=/home/logs/nginx.log --count 10

# open /home/logs/nginx.log reach the limit
blade c file load --filepath=/home/logs/nginx.log --force`,
			ActionPrograms:    []string{FileLoadBin},
			ActionCategories:  []string{category.SystemFile},
			ActionProcessHang: true,
		},
	}
}

func (*FileLoadActionCommandSpec) Name() string {
	return "load"
}

func (*FileLoadActionCommandSpec) Aliases() []string {
	return []string{"l"}
}

func (*FileLoadActionCommandSpec) ShortDesc() string {
	return "File open load"
}

func (l *FileLoadActionCommandSpec) LongDesc() string {
	if l.ActionLongDesc != "" {
		return l.ActionLongDesc
	}
	return "File open load"
}

func (*FileLoadActionCommandSpec) Categories() []string {
	return []string{category.SystemFile}
}

type FileLoadExecutor struct {
	channel spec.Channel
}

func (pl *FileLoadExecutor) Name() string {
	return "load"
}

var localChannel = channel.NewLocalChannel()

func (pl *FileLoadExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	filepath := model.ActionFlags["filepath"]
	countStr := model.ActionFlags["count"]
	force := model.ActionFlags["force"] == "true"

	if filepath == "" {
		filepath = DefaultFilePath
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return pl.stop(ctx, filepath)
	}

	response := localChannel.Run(ctx, "ulimit", "-n")
	if !response.Success {
		log.Errorf(ctx, "file load, run ulimit err: %s", response.Err)
		return spec.ResponseFailWithResult(spec.ActionNotSupport, "execute unlimit -n failed")
	}

	reStr := strings.TrimSpace(response.Result.(string))
	if reStr == "unlimited" {
		return spec.ResponseFailWithResult(spec.ActionNotSupport, "the number of open files is unlimited!")
	}

	_, err := strconv.Atoi(reStr)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, err, "the number of open files is invalid!")
	}

	if countStr == "" {
		countStr = "0"
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		log.Errorf(ctx, "count is not a number")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "is not a number")
	}
	if count < 0 {
		log.Errorf(ctx, "count < 0, count is not a illegal parameter")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "must be a positive integer")
	}
	return pl.start(ctx, filepath, count, force)
}

func (pl *FileLoadExecutor) Check(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	filepath := model.ActionFlags["filepath"]
	countStr := model.ActionFlags["count"]

	if filepath == "" {
		filepath = DefaultFilePath
	}
	if _, ok := spec.IsDestroy(ctx); ok {
		return pl.stop(ctx, filepath)
	}

	response := localChannel.Run(ctx, "ulimit", "-n")
	if !response.Success {
		log.Errorf(ctx, "file load, run ulimit err: %s", response.Err)
	}

	reStr := strings.TrimSpace(response.Result.(string))
	if reStr == "unlimited" {
		return spec.ResponseFailWithResult(spec.ActionNotSupport, "the number of open files is unlimited!")
	}

	_, err := strconv.Atoi(reStr)
	if err != nil {
		return spec.ResponseFailWithFlags(spec.ActionNotSupport, err, "the number of open files is invalid!")
	}

	if countStr == "" {
		countStr = "0"
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		log.Errorf(ctx, "count is not a number")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "is not a number")
	}
	if count < 0 {
		log.Errorf(ctx, "count < 0, count is not a illegal parameter")
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "must be a positive integer")
	}
	return spec.ReturnSuccess(ctx.Value(spec.Uid))
}

func (pl *FileLoadExecutor) SetChannel(channel spec.Channel) {
	pl.channel = channel
}

func (pl *FileLoadExecutor) start(ctx context.Context, filepath string, count int, force bool) *spec.Response {
	if !exec.CheckFilepathExists(ctx, pl.channel, filepath) {
		dir := path.Dir(filepath)
		if response := pl.channel.Run(ctx, "mkdir", fmt.Sprintf(`-p %s`, dir)); !response.Success {
			return response
		}
		log.Warnf(ctx, "`%s`: file does not exist", filepath)
		if response := pl.channel.Run(ctx, "echo", fmt.Sprintf(`%s >> %s`, filepath, filepath)); !response.Success {
			return response
		}
		// return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}
	if count == 0 {
		log.Infof(ctx, "create loop file: %s", filepath)
		go loopOpenFile(ctx, filepath, force)
		select {}
	}

	// 存储打开的文件句柄，避免被垃圾回收
	var openFiles []*os.File
	defer func() {
		// 在函数结束时关闭所有文件句柄
		for _, file := range openFiles {
			file.Close()
		}
	}()

	for i := 0; i < count; i++ {
		file, err := os.Open(filepath)
		if err != nil {
			log.Errorf(ctx, "create loop file: %s i: %d err: %s", filepath, i, err)
			break
		}
		// 将文件句柄存储到切片中，防止被垃圾回收
		openFiles = append(openFiles, file)

		// 每1000个文件句柄输出一次统计信息
		if (i+1)%1000 == 0 {
			if response := pl.channel.Run(ctx, "lsof", fmt.Sprintf("%s | wc -l", filepath)); !response.Success {
				log.Debugf(ctx, "create loop file: %s i: %d response: %v", filepath, i, response)
			} else {
				log.Debugf(ctx, "create loop file: %s i: %d response: %v", filepath, i, response)
			}
			log.Debugf(ctx, "Opened %d files, current open files: %d", i+1, len(openFiles))
		}
	}
	return spec.ReturnSuccess(ctx.Value(spec.Uid))
}

func (pl *FileLoadExecutor) stop(ctx context.Context, filepath string) *spec.Response {
	simpleProc(ctx, fmt.Sprintf("lsof %s | awk '{print $2}' | xargs kill -9", filepath))
	if filepath == DefaultFilePath && exec.CheckFilepathExists(ctx, pl.channel, filepath) {
		os.Remove(filepath)
		// return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}
	ctx = context.WithValue(ctx, "bin", FileLoadBin)
	return exec.Destroy(ctx, pl.channel, "file load")
}

func simpleProc(ctx context.Context, cmd string) {
	if err := osExec.Command("/bin/bash", "-c", cmd).Start(); err != nil {
		log.Errorf(ctx, "exec command: %s failed: %s", cmd, err)
	}
}

func loopOpenFile(ctx context.Context, filepath string, force bool) {
	// 存储打开的文件句柄，避免被垃圾回收
	var openFiles []*os.File
	defer func() {
		// 在函数结束时关闭所有文件句柄
		for _, file := range openFiles {
			file.Close()
		}
	}()

	fileCount := 0
	for {
		file, err := os.Open(filepath)
		if err != nil {
			log.Warnf(ctx, "open filepath: %s has reach the limit err: %s, total opened: %d", filepath, err, fileCount)
			if !force {
				break
			}
		}

		// 将文件句柄存储到切片中，防止被垃圾回收
		openFiles = append(openFiles, file)
		fileCount++

		// 每1000个文件句柄输出一次统计信息
		if fileCount%1000 == 0 {
			log.Debugf(ctx, "Opened %d files, current open files: %d", fileCount, len(openFiles))
		}
	}
}
