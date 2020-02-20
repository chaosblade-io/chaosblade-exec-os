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

package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

const count = 100

var burnIODirectory, burnIOSize string
var burnIORead, burnIOWrite, burnIOStart, burnIOStop, burnIONohup bool

func main() {
	flag.StringVar(&burnIODirectory, "directory", "", "the directory where the disk is burning")
	flag.StringVar(&burnIOSize, "size", "", "block size")
	flag.BoolVar(&burnIOWrite, "write", false, "write io")
	flag.BoolVar(&burnIORead, "read", false, "read io")
	flag.BoolVar(&burnIOStart, "start", false, "start burn io")
	flag.BoolVar(&burnIOStop, "stop", false, "stop burn io")
	flag.BoolVar(&burnIONohup, "nohup", false, "start by nohup")
	bin.ParseFlagAndInitLog()

	if burnIOStart {
		startBurnIO(burnIODirectory, burnIOSize, burnIORead, burnIOWrite)
	} else if burnIOStop {
		stopBurnIO(burnIODirectory, burnIORead, burnIOWrite)
	} else if burnIONohup {
		if burnIORead {
			go burnRead(burnIODirectory, burnIOSize)
		}
		if burnIOWrite {
			go burnWrite(burnIODirectory, burnIOSize)
		}
		select {}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var readFile = "chaos_burnio.read"
var writeFile = "chaos_burnio.write"
var burnIOBin = "chaos_burnio"
var logFile = util.GetNohupOutput(util.Bin, "chaos_burnio.log")

var cl = channel.NewLocalChannel()

var stopBurnIOFunc = stopBurnIO

// start burn io
func startBurnIO(directory, size string, read, write bool) {
	ctx := context.Background()
	response := cl.Run(ctx, "nohup",
		fmt.Sprintf(`%s --directory %s --size %s --read=%t --write=%t --nohup=true > %s 2>&1 &`,
			path.Join(util.GetProgramPath(), burnIOBin), directory, size, read, write, logFile))
	if !response.Success {
		stopBurnIOFunc(directory, read, write)
		bin.PrintErrAndExit(response.Err)
		return
	}
	// check
	time.Sleep(time.Second)
	response = cl.Run(ctx, "grep", fmt.Sprintf("%s %s", bin.ErrPrefix, logFile))
	if response.Success {
		errMsg := strings.TrimSpace(response.Result.(string))
		if errMsg != "" {
			stopBurnIOFunc(directory, read, write)
			bin.PrintErrAndExit(errMsg)
			return
		}
	}
	bin.PrintOutputAndExit("success")
}

// stop burn io,  no need to add os.Exit
func stopBurnIO(directory string, read, write bool) {
	ctx := context.Background()
	if read {
		// dd process
		pids, _ := cl.GetPidsByProcessName(readFile, ctx)
		if pids != nil && len(pids) > 0 {
			cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		// chaos_burnio process
		ctxWithKey := context.WithValue(ctx, channel.ProcessKey, burnIOBin)
		pids, _ = cl.GetPidsByProcessName("--read=true", ctxWithKey)
		if pids != nil && len(pids) > 0 {
			cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		cl.Run(ctx, "rm", fmt.Sprintf("-rf %s*", path.Join(directory, readFile)))
	}
	if write {
		// dd process
		pids, _ := cl.GetPidsByProcessName(writeFile, ctx)
		if pids != nil && len(pids) > 0 {
			cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		ctxWithKey := context.WithValue(ctx, channel.ProcessKey, burnIOBin)
		pids, _ = cl.GetPidsByProcessName("--write=true", ctxWithKey)
		if pids != nil && len(pids) > 0 {
			cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		cl.Run(ctx, "rm", fmt.Sprintf("-rf %s*", path.Join(directory, writeFile)))
	}
}

// write burn
func burnWrite(directory, size string) {
	tmpFileForWrite := path.Join(directory, writeFile)
	for {
		args := fmt.Sprintf(`if=/dev/zero of=%s bs=%sM count=%d oflag=dsync`, tmpFileForWrite, size, count)
		response := cl.Run(context.TODO(), "dd", args)
		if !response.Success {
			bin.PrintAndExitWithErrPrefix(response.Err)
			return
		}
	}
}

// read burn
func burnRead(directory, size string) {
	// create a 600M file under the directory
	tmpFileForRead := path.Join(directory, readFile)
	createArgs := fmt.Sprintf("if=/dev/zero of=%s bs=%dM count=%d oflag=dsync", tmpFileForRead, 6, count)
	response := cl.Run(context.TODO(), "dd", createArgs)
	if !response.Success {
		bin.PrintAndExitWithErrPrefix(
			fmt.Sprintf("using dd command to create a temp file under %s directory for reading error, %s",
				directory, response.Err))
	}
	for {
		args := fmt.Sprintf(`if=%s of=/dev/null bs=%sM count=%d iflag=dsync,direct,fullblock`, tmpFileForRead, size, count)
		response = cl.Run(context.TODO(), "dd", args)
		if !response.Success {
			bin.PrintAndExitWithErrPrefix(fmt.Sprintf("using dd command to burn read io error, %s", response.Err))
			return
		}
	}
}
