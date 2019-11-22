/*
 * Copyright 1999-2019 Alibaba Group Holding Ltd.
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

	cl "github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var fillDataFile = "chaos_filldisk.log.dat"
var fillDiskSize, fillDiskDirectory string
var fillDiskStart, fillDiskStop bool

const diskFillErrorMessage = "No space left on device"

func main() {
	flag.StringVar(&fillDiskDirectory, "directory", "", "the directory where the disk is populated")
	flag.StringVar(&fillDiskSize, "size", "", "fill size, unit is M")
	flag.BoolVar(&fillDiskStart, "start", false, "start fill or not")
	flag.BoolVar(&fillDiskStop, "stop", false, "stop fill or not")

	flag.Parse()

	if fillDiskStart == fillDiskStop {
		bin.PrintErrAndExit("must specify start or stop operation")
	}
	if fillDiskStart {
		startFill(fillDiskDirectory, fillDiskSize)
	} else if fillDiskStop {
		stopFill(fillDiskDirectory)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var channel = cl.NewLocalChannel()

func startFill(directory, size string) {
	ctx := context.TODO()
	if directory == "" {
		bin.PrintErrAndExit("--directory flag value is empty")
	}
	dataFile := path.Join(directory, fillDataFile)

	// Some normal filesystems (ext4, xfs, btrfs and ocfs2) tack quick works
	if cl.IsCommandAvailable("fallocate") {
		fillDiskByFallocate(ctx, size, dataFile)
	}
	// If execute fallocate command failed, use dd command to retry.
	fillDiskByDD(ctx, dataFile, directory, size)
}

func fillDiskByFallocate(ctx context.Context, size string, dataFile string) {
	response := channel.Run(ctx, "fallocate", fmt.Sprintf(`-l %sM %s`, size, dataFile))
	if response.Success {
		bin.PrintOutputAndExit(response.Result.(string))
	}
	// Need to judge that the disk is full or not. If the disk is full, return success
	if strings.Contains(response.Err, diskFillErrorMessage) {
		bin.PrintOutputAndExit(fmt.Sprintf("success because of %s", diskFillErrorMessage))
	}
}

func fillDiskByDD(ctx context.Context, dataFile string, directory string, size string) {
	// Because of filling disk slowly using dd, so execute dd with 1b size first to test the command.
	response := channel.Run(ctx, "dd", fmt.Sprintf(`if=/dev/zero of=%s bs=1b count=1 iflag=fullblock`, dataFile))
	if !response.Success {
		stopFill(directory)
		bin.PrintErrAndExit(response.Err)
	}
	response = channel.Run(ctx, "nohup",
		fmt.Sprintf(`dd if=/dev/zero of=%s bs=1M count=%s iflag=fullblock >/dev/null 2>&1 &`, dataFile, size))
	if !response.Success {
		stopFill(directory)
		bin.PrintErrAndExit(response.Err)
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

// stopFill contains kill the filldisk process and delete the temp file actions
func stopFill(directory string) {
	ctx := context.Background()
	pids, _ := cl.GetPidsByProcessName(fillDataFile, ctx)
	if pids != nil || len(pids) >= 0 {
		channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
	}
	fileName := path.Join(directory, fillDataFile)
	if util.IsExist(fileName) {
		response := channel.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, fileName))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
	}
}
