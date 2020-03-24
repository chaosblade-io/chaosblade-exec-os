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
	"math"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var fillDataFile = "chaos_filldisk.log.dat"
var fillDiskSize, fillDiskDirectory, fillDiskPercent, ReserveDiskSize string
var fillDiskStart, fillDiskStop bool

const diskFillErrorMessage = "No space left on device"

func main() {
	flag.StringVar(&fillDiskDirectory, "directory", "", "the directory where the disk is populated")
	flag.StringVar(&fillDiskSize, "size", "", "fill size, unit is M")
	flag.StringVar(&ReserveDiskSize, "reserve", "", "reserve size, unit is M")
	flag.StringVar(&fillDiskPercent, "percent", "", "percentage of disk, positive integer without %")
	flag.BoolVar(&fillDiskStart, "start", false, "start fill or not")
	flag.BoolVar(&fillDiskStop, "stop", false, "stop fill or not")
	bin.ParseFlagAndInitLog()

	if fillDiskStart == fillDiskStop {
		bin.PrintErrAndExit("must specify start or stop operation")
	}
	if fillDiskStart {
		startFill(fillDiskDirectory, fillDiskSize, fillDiskPercent, ReserveDiskSize)
	} else if fillDiskStop {
		stopFill(fillDiskDirectory)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func startFill(directory, size, percent, reserve string) {
	ctx := context.TODO()
	if directory == "" {
		bin.PrintErrAndExit("--directory flag value is empty")
	}
	if size == "" && percent == "" && reserve == "" {
		bin.PrintErrAndExit("less --size or --percent or --reserve  flag")
	}
	dataFile := path.Join(directory, fillDataFile)
	// calculate the fill size
	size, err := calculateFileSize(directory, size, percent, reserve)
	if err != nil {
		bin.PrintErrAndExit(fmt.Sprintf("calculate size err, %v", err))
	}

	// Some normal filesystems (ext4, xfs, btrfs and ocfs2) tack quick works
	if cl.IsCommandAvailable("fallocate") {
		fillDiskByFallocate(ctx, size, dataFile)
	}
	// If execute fallocate command failed, use dd command to retry.
	fillDiskByDD(ctx, dataFile, directory, size)
}

// calculateFileSize returns the size which should be filled, unit is M
func calculateFileSize(directory, size, percent, reserve string) (string, error) {
	if percent == "" && reserve == "" {
		return size, nil
	}

	var stat syscall.Statfs_t
	syscall.Statfs(directory, &stat)
	allBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := allBytes - availableBytes

	if percent != "" {
		p, err := strconv.Atoi(percent)
		if err != nil {
			return "", err
		}

		usedPercentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(usedBytes)/float64(allBytes)), 64)
		expectedPercentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(p)/100.0), 64)

		if usedPercentage >= expectedPercentage {
			return "", fmt.Errorf("the disk has been used %.2f, large than expected", usedPercentage)
		}
		remainderPercentage := expectedPercentage - usedPercentage
		logrus.Debugf("remainderPercentage: %f", remainderPercentage)
		expectSize := math.Floor(remainderPercentage * float64(allBytes) / (1024.0 * 1024.0))
		return fmt.Sprintf("%.f", expectSize), nil
	} else {
		r, err := strconv.ParseFloat(reserve, 64)
		if err != nil {
			return "", err
		}

		availableMB := float64(availableBytes) / (1024.0 * 1024.0)
		if availableMB <= r {
			return "", fmt.Errorf("the disk has available size %.2f, less than expected", availableMB)
		}
		expectSize := math.Floor(availableMB - r)
		return fmt.Sprintf("%.f", expectSize), nil
	}
}

func fillDiskByFallocate(ctx context.Context, size string, dataFile string) {
	response := cl.Run(ctx, "fallocate", fmt.Sprintf(`-l %sM %s`, size, dataFile))
	if response.Success {
		bin.PrintOutputAndExit(response.Result.(string))
	}
	// Need to judge that the disk is full or not. If the disk is full, return success
	if strings.Contains(response.Err, diskFillErrorMessage) {
		bin.PrintOutputAndExit(fmt.Sprintf("success because of %s", diskFillErrorMessage))
	}
	logrus.Warningf("execute fallocate err, %s", response.Err)
}

func fillDiskByDD(ctx context.Context, dataFile string, directory string, size string) {
	// Because of filling disk slowly using dd, so execute dd with 1b size first to test the command.
	response := cl.Run(ctx, "dd", fmt.Sprintf(`if=/dev/zero of=%s bs=1b count=1 iflag=fullblock`, dataFile))
	if !response.Success {
		stopFill(directory)
		bin.PrintErrAndExit(response.Err)
	}
	response = cl.Run(ctx, "nohup",
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
	if directory == "" {
		bin.PrintErrAndExit("--directory flag value is empty")
	}
	pids, _ := cl.GetPidsByProcessName(fillDataFile, ctx)
	if pids != nil || len(pids) >= 0 {
		cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
	}
	fileName := path.Join(directory, fillDataFile)
	if util.IsExist(fileName) {
		response := cl.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, fileName))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
	}
}
