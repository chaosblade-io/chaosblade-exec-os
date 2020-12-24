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
	"errors"
	"flag"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/disk"
)

const count = 100

type DiskBurnExp struct {
	Path  string
	Size  string
	Read  bool
	Write bool
	Start bool
	Stop  bool
	Nohup bool

	TempFileForReadName  string
	TempFileForWriteName string
	BurnIOBinName        string
	LogFile              string
	Channel              channel.OsChannel
}

var expObj = DiskBurnExp{
	TempFileForReadName:  "chaos_burnio.read",
	TempFileForWriteName: "chaos_burnio.write",
	BurnIOBinName:        exec.BurnIOBin,
	LogFile:              util.GetNohupOutput(util.Bin, "chaos_burnio.log"),
	Channel:              channel.NewLocalChannel(),
}

func main() {
	flag.CommandLine = flag.NewFlagSet("disk", flag.ContinueOnError)
	flag.StringVar(&expObj.Path, disk.PathFlag.Name, "/", disk.PathFlag.Desc)
	flag.StringVar(&expObj.Size, disk.SizeFlag.Name, "10", disk.SizeFlag.Desc)
	flag.BoolVar(&expObj.Write, disk.WriteFlag.Name, false, disk.WriteFlag.Desc)
	flag.BoolVar(&expObj.Read, disk.ReadFlag.Name, false, disk.ReadFlag.Desc)
	flag.BoolVar(&expObj.Start, disk.StartFlag.Name, false, disk.StartFlag.Desc)
	flag.BoolVar(&expObj.Stop, disk.StopFlag.Name, false, disk.StopFlag.Desc)
	flag.BoolVar(&expObj.Nohup, disk.NohupFlag.Name, false, disk.NohupFlag.Desc)

	bin.ParseFlagAndInitLog()

	Exec(expObj)
}

func Exec(expObj DiskBurnExp) {
	if err := validateFlags(expObj); err != nil {
		bin.PrintAndExitWithErrPrefix(err.Error())
	}

	if expObj.Start {
		expObj.startBurnIO()
	} else if expObj.Stop {
		expObj.stopBurnIO()
	} else if expObj.Nohup {
		if expObj.Read {
			go expObj.burnRead()
		}
		if expObj.Write {
			go expObj.burnWrite()
		}
		select {}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

func validateFlags(expObj DiskBurnExp) error {
	if expObj.Start == expObj.Stop && !expObj.Nohup {
		return errors.New("must specify only one flag between start and stop flags")
	}
	err := disk.CheckDiskExpEnv()
	if err != nil {
		return err
	}
	if expObj.Channel == nil {
		return errors.New("channel is nil")
	}
	if expObj.Stop {
		// set readExists and writeExists to true if does not specify read and write flags
		if expObj.Read == false && expObj.Write == false {
			expObj.Read = true
			expObj.Write = true
		}
		return nil
	}
	if !util.IsDir(expObj.Path) {
		return fmt.Errorf("the %s path must be directory", expObj.Path)
	}
	if expObj.Read == false && expObj.Write == false {
		return fmt.Errorf("less --read or --write flag")
	}
	return nil
}

// start burn io
func (d *DiskBurnExp) startBurnIO() {
	ctx := context.Background()
	response := d.Channel.Run(ctx, "nohup",
		fmt.Sprintf(`%s --path %s --size %s --read=%t --write=%t --nohup=true > %s 2>&1 &`,
			path.Join(util.GetProgramPath(), d.BurnIOBinName), d.Path, d.Size, d.Read, d.Write, d.LogFile))
	if !response.Success {
		d.stopBurnIO()
		bin.PrintErrAndExit(response.Err)
		return
	}
	// check
	time.Sleep(time.Second)
	response = d.Channel.Run(ctx, "grep", fmt.Sprintf("%s %s", bin.ErrPrefix, d.LogFile))
	if response.Success {
		errMsg := strings.TrimSpace(response.Result.(string))
		if errMsg != "" {
			d.stopBurnIO()
			bin.PrintErrAndExit(errMsg)
			return
		}
	}
	bin.PrintOutputAndExit("success")
}

// stop burn io,  no need to add os.Exit
func (d *DiskBurnExp) stopBurnIO() {
	ctx := context.WithValue(context.Background(), channel.ExcludeProcessKey, "--stop")
	if d.Read {
		// dd process
		pids, _ := d.Channel.GetPidsByProcessName(d.TempFileForReadName, ctx)
		if pids != nil && len(pids) > 0 {
			d.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		// chaos_burnio process
		ctxWithKey := context.WithValue(ctx, channel.ProcessKey, d.BurnIOBinName)
		pids, _ = d.Channel.GetPidsByProcessName("--read=true", ctxWithKey)
		if pids != nil && len(pids) > 0 {
			d.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		d.Channel.Run(ctx, "rm", fmt.Sprintf("-rf %s*", path.Join(d.Path, d.TempFileForReadName)))
	}
	if d.Write {
		// dd process
		pids, _ := d.Channel.GetPidsByProcessName(d.TempFileForWriteName, ctx)
		if pids != nil && len(pids) > 0 {
			d.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		ctxWithKey := context.WithValue(ctx, channel.ProcessKey, d.BurnIOBinName)
		pids, _ = d.Channel.GetPidsByProcessName("--write=true", ctxWithKey)
		if pids != nil && len(pids) > 0 {
			d.Channel.Run(ctx, "kill", fmt.Sprintf("-9 %s", strings.Join(pids, " ")))
		}
		d.Channel.Run(ctx, "rm", fmt.Sprintf("-rf %s*", path.Join(d.Path, d.TempFileForWriteName)))
	}
}

// write burn
func (d *DiskBurnExp) burnWrite() {
	tmpFileForWrite := path.Join(d.Path, d.TempFileForWriteName)
	for {
		args := fmt.Sprintf(`if=/dev/zero of=%s bs=%sM count=%d oflag=dsync`, tmpFileForWrite, d.Size, count)
		response := d.Channel.Run(context.TODO(), "dd", args)
		if !response.Success {
			bin.PrintAndExitWithErrPrefix(response.Err)
			return
		}
	}
}

// read burn
func (d *DiskBurnExp) burnRead() {
	// create a 600M file under the directory
	tmpFileForRead := path.Join(d.Path, d.TempFileForReadName)
	createArgs := fmt.Sprintf("if=/dev/zero of=%s bs=%dM count=%d oflag=dsync", tmpFileForRead, 6, count)
	response := d.Channel.Run(context.TODO(), "dd", createArgs)
	if !response.Success {
		bin.PrintAndExitWithErrPrefix(
			fmt.Sprintf("using dd command to create a temp file under %s directory for reading error, %s",
				d.Path, response.Err))
	}
	for {
		args := fmt.Sprintf(`if=%s of=/dev/null bs=%sM count=%d iflag=dsync,direct,fullblock`, tmpFileForRead, d.Size, count)
		response = d.Channel.Run(context.TODO(), "dd", args)
		if !response.Success {
			bin.PrintAndExitWithErrPrefix(fmt.Sprintf("using dd command to burn read io error, %s", response.Err))
			return
		}
	}
}
