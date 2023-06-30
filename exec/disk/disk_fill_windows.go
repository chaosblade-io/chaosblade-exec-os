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
	"math"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/shirou/gopsutil/disk"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

/*
*
fsutil
Applies to: Windows Server 2022, Windows Server 2019, Windows Server 2016, Windows 10, Windows Server 2012 R2,
Windows 8.1, Windows Server 2012, Windows 8, Windows Server 2008 R2, Windows 7

eg: fsutil file createNew temp-1GB-file 1000000000 // uint byte
*/
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
					Desc: "The path of directory where the disk is populated, default value is C:\\",
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
blade create disk fill --path C:\Users --size 40000

# Performs populating the disk by percentage, and retains the file handle that populates the disk
Command: "blade c disk fill --path C:\Users --percent 80 --retain-handle

# Perform a fixed-size experimental scenario
blade c disk fill --path C:\Users --reserve 1024`,
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
	directory := "C:\\"
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
	// fsutil file createNew temp-1GB-file 1000000000
	response := fillDiskByFsutil(ctx, size, dataFile, cl)

	if response.Success {
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
func fillDiskByFsutil(ctx context.Context, size string, dataFile string, cl spec.Channel) *spec.Response {
	response := cl.Run(ctx, "fsutil", fmt.Sprintf(`file createNew %s %s`, dataFile, size))
	if response.Success {
		return response
	}
	// Need to judge that the disk is full or not. If the disk is full, return success
	if strings.Contains(response.Err, diskFillErrorMessage) {
		return spec.ReturnSuccess(fmt.Sprintf("success because of %s", diskFillErrorMessage))
	}
	log.Warnf(ctx, "execute fsutil err, %s", response.Err)
	return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "fsutil", response.Err)
}

// calculateFileSize returns the size which should be filled, unit is M
func calculateFileSize(ctx context.Context, directory, size, percent, reserve string) (string, error) {
	if percent == "" && reserve == "" {
		isize, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			return "", fmt.Errorf("get size failed")
		}
		return strconv.FormatInt(isize*1024*1024, 10), nil
	}
	stat, _ := disk.Usage(directory)

	allBytes := stat.Total
	availableBytes := stat.Free
	usedBytes := stat.Used

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
		log.Debugf(ctx, "remainderPercentage: %f", remainderPercentage)
		expectSize := math.Floor(remainderPercentage * float64(allBytes)) // / (1024.0 * 1024.0)
		return fmt.Sprintf("%.f", expectSize), nil
	} else {
		r, err := strconv.ParseFloat(reserve, 64)
		if err != nil {
			return "", err
		}
		r = r * 1024.0 * 1024.0
		availableMB := float64(availableBytes)
		if availableMB <= r {
			return "", fmt.Errorf("the disk has available size %.2f, less than expected", availableMB)
		}
		expectSize := math.Floor(availableMB - r)
		return fmt.Sprintf("%.f", expectSize), nil
	}
}

// stopFill contains kill the filldisk process and delete the temp file actions
func stopFill(ctx context.Context, directory string, cl spec.Channel) *spec.Response {

	if directory == "" {
		log.Errorf(ctx, "`%s`: directory is nil", directory)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "directory", directory, "directory is nil")
	}
	// kill daemon process
	//todo
	//ctx = context.WithValue(ctx, channel.ProcessKey, fillDiskBin)
	pids, _ := cl.GetPidsByProcessName("disk fill", ctx)
	if pids != nil && len(pids) >= 0 {
		resp := cl.Kill(ctx, pids)
		log.Errorf(ctx, "kill disk fill daemon process err: %s", resp.Err)
	}

	fileName := path.Join(directory, fillDataFile)
	if exec.CheckFilepathExists(ctx, cl, fileName) {
		err := os.Remove(fileName)
		if err != nil {
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "remove file", err.Error())
		}
	}
	return spec.Success()
}
