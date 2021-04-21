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

package burnio

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
)

// init registry provider to model.
func init() {
	model.Provide(new(BurnIO))
}

type BurnIO struct {
	BurnIODirectory string `name:"directory" json:"directory" yaml:"directory" default:"" help:"the directory where the disk is burning"`
	BurnIOSize      string `name:"size" json:"size" yaml:"size" default:"" help:"block size"`
	BurnIOWrite     bool   `name:"write" json:"write" yaml:"write" default:"false" help:"write io"`
	BurnIORead      bool   `name:"read" json:"read" yaml:"read" default:"false" help:"read io"`
	BurnIOStart     bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start burn io"`
	BurnIOStop      bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop burn io"`
	BurnIONohup     bool   `name:"nohup" json:"nohup" yaml:"nohup" default:"false" help:"start by nohup"`

	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
	StartBurnIO func(directory, size string, read, write bool) `kong:"-"`
	StopBurnIO  func(directory string, read, write bool)       `kong:"-"`
}

func (that *BurnIO) Assign() model.Worker {
	worker := &BurnIO{Channel: channel.NewLocalChannel()}
	worker.StartBurnIO = func(directory, size string, read, write bool) {
		worker.startBurnIO(directory, size, read, write)
	}
	worker.StopBurnIO = func(directory string, read, write bool) {
		worker.stopBurnIO(directory, read, write)
	}
	return worker
}

func (that *BurnIO) Name() string {
	return exec.BurnIOBin
}

func (that *BurnIO) Exec() *spec.Response {
	if that.BurnIOStart {
		that.StartBurnIO(that.BurnIODirectory, that.BurnIOSize, that.BurnIORead, that.BurnIOWrite)
	} else if that.BurnIOStop {
		that.StopBurnIO(that.BurnIODirectory, that.BurnIORead, that.BurnIOWrite)
	} else if that.BurnIONohup {
		if that.BurnIORead {
			go that.burnRead(that.BurnIODirectory, that.BurnIOSize)
		}
		if that.BurnIOWrite {
			go that.burnWrite(that.BurnIODirectory, that.BurnIOSize)
		}
		select {}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

const count = 100

var readFile = "chaos_burnio.read"
var writeFile = "chaos_burnio.write"
var logFile = util.GetNohupOutput(util.Bin, "chaos_burnio.log")

// start burn io
func (that *BurnIO) startBurnIO(directory, size string, read, write bool) {
	ctx := context.Background()
	response := that.Channel.Run(ctx, "nohup",
		fmt.Sprintf(`%s --directory %s --size %s --read=%t --write=%t --nohup=true > %s 2>&1 &`,
			path.Join(util.GetProgramPath(), that.Name()), directory, size, read, write, logFile))
	if !response.Success {
		that.StopBurnIO(directory, read, write)
		bin.PrintErrAndExit(response.Err)
		return
	}
	// check
	time.Sleep(time.Second)
	response = that.Channel.Run(ctx, "grep", fmt.Sprintf("%s %s", bin.ErrPrefix, logFile))
	if response.Success {
		errMsg := strings.TrimSpace(response.Result.(string))
		if errMsg != "" {
			that.StopBurnIO(directory, read, write)
			bin.PrintErrAndExit(errMsg)
			return
		}
	}
	bin.PrintOutputAndExit("success")
}

// stop burn io,  no need to add os.Exit
func (that *BurnIO) stopBurnIO(directory string, read, write bool) {
	ctx := context.WithValue(context.Background(), channel.ExcludeProcessKey, "--stop")
	if read {
		// dd process
		pids, _ := that.Channel.GetPidsByProcessName(readFile, ctx)
		if pids != nil && len(pids) > 0 {
			that.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		// chaos_burnio process
		ctxWithKey := context.WithValue(ctx, channel.ProcessKey, that.Name())
		pids, _ = that.Channel.GetPidsByProcessName("--read=true", ctxWithKey)
		if pids != nil && len(pids) > 0 {
			that.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		that.Channel.Run(ctx, "rm", fmt.Sprintf("-rf %s*", path.Join(directory, readFile)))
	}
	if write {
		// dd process
		pids, _ := that.Channel.GetPidsByProcessName(writeFile, ctx)
		if pids != nil && len(pids) > 0 {
			that.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		ctxWithKey := context.WithValue(ctx, channel.ProcessKey, that.Name())
		pids, _ = that.Channel.GetPidsByProcessName("--write=true", ctxWithKey)
		if pids != nil && len(pids) > 0 {
			that.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		that.Channel.Run(ctx, "rm", fmt.Sprintf("-rf %s*", path.Join(directory, writeFile)))
	}
}

// write burn
func (that *BurnIO) burnWrite(directory, size string) {
	if !that.Channel.IsCommandAvailable("dd") {
		bin.PrintErrAndExit(spec.ResponseErr[spec.CommandDdNotFound].Err)
	}
	tmpFileForWrite := path.Join(directory, writeFile)
	for {
		args := fmt.Sprintf(`if=/dev/zero of=%s bs=%sM count=%d oflag=dsync`, tmpFileForWrite, size, count)
		response := that.Channel.Run(context.TODO(), "dd", args)
		if !response.Success {
			bin.PrintAndExitWithErrPrefix(response.Err)
			return
		}
	}
}

// read burn
func (that *BurnIO) burnRead(directory, size string) {
	// create a 600M file under the directory
	tmpFileForRead := path.Join(directory, readFile)
	createArgs := fmt.Sprintf("if=/dev/zero of=%s bs=%dM count=%d oflag=dsync", tmpFileForRead, 6, count)
	response := that.Channel.Run(context.TODO(), "dd", createArgs)
	if !response.Success {
		bin.PrintAndExitWithErrPrefix(
			fmt.Sprintf("using dd command to create a temp file under %s directory for reading error, %s",
				directory, response.Err))
	}
	for {
		args := fmt.Sprintf(`if=%s of=/dev/null bs=%sM count=%d iflag=dsync,direct,fullblock`, tmpFileForRead, size, count)
		response = that.Channel.Run(context.TODO(), "dd", args)
		if !response.Success {
			bin.PrintAndExitWithErrPrefix(fmt.Sprintf("using dd command to burn read io error, %s", response.Err))
			return
		}
	}
}
