/*
 * Copyright 2025 The ChaosBlade Authors
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
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
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
Command: "blade create disk fill --path /home --percent 80 --retain-handle

# Perform a fixed-size experimental scenario
blade create disk fill --path /home --reserve 1024`,
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
	directory := "/"
	path := model.ActionFlags["path"]
	if path != "" {
		directory = path
	}
	if !util.IsDir(directory) {
		log.Errorf(ctx, "`%s`: path is illegal, is not a directory", directory)
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
					log.Errorf(ctx, "`%s`: size is illegal, it must be positive integer", size)
					return spec.ResponseFailWithFlags(spec.ParameterIllegal, "size", size, "it must be positive integer")
				}
				return fae.start(uid, directory, size, percent, reserve, retainHandle, ctx)
			}
			_, err := strconv.Atoi(reserve)
			if err != nil {
				log.Errorf(ctx, "`%s`: reserve is illegal, it must be positive integer", reserve)
				return spec.ResponseFailWithFlags(spec.ParameterIllegal, "reserve", reserve, "it must be positive integer")
			}
			return fae.start(uid, directory, "", percent, reserve, retainHandle, ctx)
		}
		_, err := strconv.Atoi(percent)
		if err != nil {
			log.Errorf(ctx, "`%s`: percent is illegal, it must be positive integer", percent)
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "percent", percent, "it must be positive integer")
		}
		return fae.start(uid, directory, "", percent, "", retainHandle, ctx)
	}
}

func (fae *FillActionExecutor) start(uid, directory, size, percent, reserve string, retainHandle bool, ctx context.Context) *spec.Response {
	return startFill(ctx, uid, directory, size, percent, reserve, retainHandle, fae.channel)
}

func (fae *FillActionExecutor) stop(directory string, ctx context.Context) *spec.Response {
	return stopFill(ctx, directory, fae.channel)
}

func (fae *FillActionExecutor) SetChannel(channel spec.Channel) {
	fae.channel = channel
}

var fillDataFile = "chaos_filldisk.log.dat"

// retainFileHandle by opening the file
func retainFileHandle(ctx context.Context, cl spec.Channel, fillDiskDirectory string) *spec.Response {
	// open the temp file to retain file handle
	dataFilePath := path.Join(fillDiskDirectory, fillDataFile)
	file, err := os.Open(dataFilePath)
	if err != nil {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("failed to read %s file, %s", dataFilePath, err.Error()))
	}
	defer file.Close()
	select {}
}

const diskFillErrorMessage = "No space left on device"

func startFill(ctx context.Context, uid, directory, size, percent, reserve string, retainHandle bool, cl spec.Channel) *spec.Response {
	if directory == "" {
		log.Errorf(ctx, "`%s`: directory is nil", directory)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "directory", directory, "directory is nil")
	}
	if size == "" && percent == "" && reserve == "" {
		log.Errorf(ctx, "`%s`: less --size or --percent or --reserve flag", directory)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "directory", directory, "less --size or --percent or --reserve flag")
	}
	dataFile := path.Join(directory, fillDataFile)
	size, err := calculateFileSize(ctx, directory, size, percent, reserve)
	if err != nil {
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("calculate size err, %v", err))
	}
	var response *spec.Response
	// Some normal filesystems (ext4, xfs, btrfs and ocfs2) tack quick works
	if cl.IsCommandAvailable(ctx, "fallocate") {
		response = fillDiskByFallocate(ctx, size, dataFile, cl)
	}
	if response == nil || !response.Success {
		// If execute fallocate command failed, use dd command to retry.
		response = fillDiskByDD(ctx, dataFile, directory, size, cl)
	}
	if response.Success {
		if percent != "" {
			// The one-shot fill only enforces the target at create time; space reclaimed
			// afterwards (e.g. log rotation) would become permanent headroom. Keep a
			// detached maintainer topping the disk back up until destroy.
			if err := startFillMaintainer(ctx, directory, percent, cl); err != nil {
				log.Warnf(ctx, "failed to start fill maintainer, %v", err)
			}
		}
		if retainHandle {
			// start a process to hold the file handle
			response := retainFileHandle(ctx, cl, directory)
			if !response.Success {
				return response
			}
		}
		return response
	}
	if err = stopFill(ctx, directory, cl); err != nil {
		log.Warnf(ctx, "failed to stop fill when starting failed, %v, starting err: %s", err, response.Err)
	}
	return response
}

var getSysStatFunc = func(directory string) *syscall.Statfs_t {
	var stat syscall.Statfs_t
	syscall.Statfs(directory, &stat)
	return &stat
}

// calculateFileSize returns the size which should be filled, unit is M
func calculateFileSize(ctx context.Context, directory, size, percent, reserve string) (string, error) {
	if percent == "" && reserve == "" {
		return size, nil
	}
	stat := getSysStatFunc(directory)
	freeBytes := stat.Bfree * uint64(stat.Bsize)

	if percent != "" {
		p, err := strconv.Atoi(percent)
		if err != nil {
			return "", err
		}
		deficit, err := fillDeficitBytes(stat, p)
		if err != nil {
			return "", err
		}
		log.Debugf(ctx, "fill deficit bytes: %d", deficit)
		return fmt.Sprintf("%.f", math.Floor(float64(deficit)/(1024.0*1024.0))), nil
	} else {
		r, err := strconv.ParseFloat(reserve, 64)
		if err != nil {
			return "", err
		}
		availableMB := float64(freeBytes) / (1024.0 * 1024.0)
		if availableMB <= r {
			return "", fmt.Errorf("the disk has available size %.2f, less than expected", availableMB)
		}
		expectSize := math.Floor(availableMB - r)
		return fmt.Sprintf("%.f", expectSize), nil
	}
}

func fillDiskByFallocate(ctx context.Context, size string, dataFile string, cl spec.Channel) *spec.Response {
	response := cl.Run(ctx, "fallocate", fmt.Sprintf(`-l %sM %s`, size, dataFile))
	if response.Success {
		return response
	}
	// Need to judge that the disk is full or not. If the disk is full, return success
	if strings.Contains(response.Err, diskFillErrorMessage) {
		return spec.ReturnSuccess(fmt.Sprintf("success because of %s", diskFillErrorMessage))
	}
	log.Warnf(ctx, "execute fallocate err, %s", response.Err)
	return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "fallocate", response.Err)
}

func fillDiskByDD(ctx context.Context, dataFile string, directory string, size string, cl spec.Channel) *spec.Response {
	if !cl.IsCommandAvailable(ctx, "dd") {
		return spec.ResponseFailWithFlags(spec.CommandDdNotFound)
	}

	// Because of filling disk slowly using dd, so execute dd with 1b size first to test the command.
	response := cl.Run(ctx, "dd", fmt.Sprintf(`if=/dev/zero of=%s bs=1b count=1 iflag=fullblock`, dataFile))
	if !response.Success {
		return response
	}
	return cl.Run(ctx, "nohup",
		fmt.Sprintf(`dd if=/dev/zero of=%s bs=1M count=%s iflag=fullblock >/dev/null 2>&1 &`, dataFile, size))
}

// stopFill contains kill the filldisk process and delete the temp file actions
func stopFill(ctx context.Context, directory string, cl spec.Channel) *spec.Response {
	if directory == "" {
		log.Errorf(ctx, "`%s`: directory is nil", directory)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "directory", directory, "directory is nil")
	}
	// kill dd or fallocate process
	pids, _ := cl.GetPidsByProcessName(fillDataFile, ctx)
	if pids != nil && len(pids) > 0 {
		resp := cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		if !resp.Success {
			log.Errorf(ctx, "kill fallocate process err: %s", resp.Err)
		}
	}
	// kill daemon process
	// todo
	// ctx = context.WithValue(ctx, channel.ProcessKey, fillDiskBin)
	pids, _ = cl.GetPidsByProcessName("disk fill", ctx)
	if pids != nil && len(pids) > 0 {
		resp := cl.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		if !resp.Success {
			log.Errorf(ctx, "kill disk fill daemon process err: %s", resp.Err)
		}
	}
	fileName := path.Join(directory, fillDataFile)
	if exec.CheckFilepathExists(ctx, cl, fileName) {
		return cl.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, fileName))
	}
	return spec.Success()
}

// MaintainMode is the hidden bin mode that runs the sustained fill maintainer.
const MaintainMode = "maintain"

// fillMaintainInterval is how often the maintainer re-checks disk usage.
const fillMaintainInterval = 2 * time.Second

// fillDeficitBytes returns the bytes needed to bring true disk usage up to percent.
// It uses Bfree (all free blocks, including root-reserved) so reserved space is not
// miscounted as used, and works in bytes to avoid percentage-rounding headroom.
func fillDeficitBytes(stat *syscall.Statfs_t, percent int) (int64, error) {
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)
	used := total - free
	target := total * int64(percent) / 100
	deficit := target - used
	if deficit <= 0 {
		return 0, fmt.Errorf("the disk has been used >= %d%%, no fill needed", percent)
	}
	if deficit > free {
		deficit = free
	}
	return deficit, nil
}

// startFillMaintainer launches a detached process running this binary in MaintainMode
// so the target usage is maintained after the create command returns.
func startFillMaintainer(ctx context.Context, directory, percent string, cl spec.Channel) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	resp := cl.Run(ctx, "nohup", fmt.Sprintf("%s %s disk fill --path=%s --percent=%s >/dev/null 2>&1 &", self, MaintainMode, directory, percent))
	if !resp.Success {
		return fmt.Errorf("%s", resp.Err)
	}
	return nil
}

// MaintainFill blocks, periodically topping the fill file up so disk usage stays at
// percent until the fill file is removed (destroy) or the process is killed.
func MaintainFill(ctx context.Context, directory, percent string) {
	p, err := strconv.Atoi(percent)
	if err != nil {
		log.Errorf(ctx, "maintain: invalid percent %s", percent)
		return
	}
	dataFile := path.Join(directory, fillDataFile)
	for {
		if !exec.CheckFilepathExists(ctx, localChannel, dataFile) {
			return
		}
		stat := getSysStatFunc(directory)
		deficit, err := fillDeficitBytes(stat, p)
		if err == nil && deficit > 0 {
			topUpFill(ctx, dataFile, deficit)
		}
		time.Sleep(fillMaintainInterval)
	}
}

// topUpFill grows the fill file by deficit bytes to reclaim freed space.
func topUpFill(ctx context.Context, dataFile string, deficit int64) {
	info, err := os.Stat(dataFile)
	if err != nil {
		return
	}
	newSizeMB := (info.Size() + deficit) / (1024 * 1024)
	resp := fillDiskByFallocate(ctx, fmt.Sprintf("%d", newSizeMB), dataFile, localChannel)
	if resp == nil || !resp.Success {
		// fallocate unavailable/failed: append with dd instead.
		deficitMB := deficit / (1024 * 1024)
		if deficitMB > 0 {
			localChannel.Run(ctx, "dd", fmt.Sprintf("if=/dev/zero of=%s bs=1M count=%d oflag=append conv=notrunc iflag=fullblock", dataFile, deficitMB))
		}
	}
}
