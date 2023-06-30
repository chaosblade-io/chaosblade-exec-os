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
	"os"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

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
					Desc: "The path of directory where the disk is burning, default value is C:\\Users",
				},
			},
			ActionExecutor: &BurnIOExecutor{},
			ActionExample: `
# The data of rkB/s, wkB/s and % Util were mainly observed. Perform disk read IO high-load scenarios
blade create disk burn --read --path C:\Users

# Perform disk write IO high-load scenarios
blade create disk burn --write --path C:\Users

# Read and write IO load scenarios are performed at the same time. Path is not specified. The default is C:\
blade create disk burn --read --write`,
			ActionPrograms:    []string{BurnIOBin},
			ActionCategories:  []string{category.SystemDisk},
			ActionProcessHang: true,
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

var localChannel = channel.NewLocalChannel()

func (be *BurnIOExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {

	directory := model.ActionFlags["path"]
	if directory == "" {
		directory = "C:\\"
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
		log.Errorf(ctx, "`%s`: path is illegal, is not a directory", directory)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "path", directory, "it must be a directory")
	}
	readExists := model.ActionFlags["read"] == "true"
	writeExists := model.ActionFlags["write"] == "true"
	if !readExists && !writeExists {
		log.Errorf(ctx, "less params, read|write")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "read|write")
	}
	size := model.ActionFlags["size"]
	if size == "" {
		size = "10"
	}
	return be.start(ctx, readExists, writeExists, directory, size)
}

func (be *BurnIOExecutor) start(ctx context.Context, read, write bool, directory, size string) *spec.Response {
	if read {
		go burnRead(ctx, directory, size, be.channel)
	}
	if write {
		go burnWrite(ctx, directory, size, be.channel)
	}
	select {}
}

func (be *BurnIOExecutor) stop(ctx context.Context, read, write bool, directory string) *spec.Response {
	if read {
		err := os.Remove(path.Join(directory, readFile))
		if err != nil {
			log.Errorf(ctx, "clean read file: %s", err.Error())
		}
	}
	if write {
		err := os.Remove(path.Join(directory, writeFile))
		if err != nil {
			log.Errorf(ctx, "clean write file: %s", err.Error())
		}
	}
	ctx = context.WithValue(ctx, "bin", BurnIOBin)
	return exec.Destroy(ctx, be.channel, "disk burn")
}

func (be *BurnIOExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}

var readFile = "chaos_burnio.read"
var writeFile = "chaos_burnio.write"

const count = 100

// write burn
func burnWrite(ctx context.Context, directory, size string, cl spec.Channel) {
	tmpFileForWrite := path.Join(directory, writeFile)
	_, _, ddRunningWriteArg := getArgs(ctx)
	for {
		args := fmt.Sprintf(ddRunningWriteArg, tmpFileForWrite, size, count)
		response := localChannel.Run(ctx, "dd", args)
		if !response.Success {
			log.Errorf(ctx, "disk burn write, run dd err: %s", response.Err)
			break
		}
	}
}

// read burn
func burnRead(ctx context.Context, directory, size string, cl spec.Channel) {
	// create a 600M file under the directory
	tmpFileForRead := path.Join(directory, readFile)
	ddCreateArg, ddRunningReadArg, _ := getArgs(ctx)
	createArgs := fmt.Sprintf(ddCreateArg, tmpFileForRead, 6, count)
	response := localChannel.Run(ctx, "dd", createArgs)
	if !response.Success {
		log.Errorf(ctx, "disk burn read, run dd err: %s", response.Err)
	}

	for {
		args := fmt.Sprintf(ddRunningReadArg, tmpFileForRead, size, count)
		//run with local channel
		response := localChannel.Run(ctx, "dd", args)
		if !response.Success {
			log.Errorf(ctx, "disk burn read, run dd err: %s", response.Err)
			break
		}
	}
}

// todo ,这里用nul的话，并不能试下真是的磁盘io占用的效果
func getArgs(ctx context.Context) (string, string, string) {
	createArgs := "if=NUL of=%s bs=%dM count=%d oflag=sync"
	runningReadArgs := "if=%s of=NUL bs=%sM count=%d iflag=sync"
	runningWriteArgs := "if=NUL of=%s bs=%sM count=%d oflag=sync"
	return createArgs, runningReadArgs, runningWriteArgs
}
