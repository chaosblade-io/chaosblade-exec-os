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
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const UnmountStuckBin = "chaos_unmountstuck"

var unmountStuckFile = "chaos_unmountstuck.dat"

type UnmountStuckActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewUnmountStuckActionSpec() spec.ExpActionCommandSpec {
	return &UnmountStuckActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "path",
					Desc:     "The mount point path to hold file handles on, making umount fail with device busy",
					Required: true,
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &UnmountStuckExecutor{},
			ActionExample: `
# Simulate volume unmount stuck on /mnt/data
blade create disk unmount_stuck --path /mnt/data

# Destroy the experiment
blade destroy disk unmount_stuck --uid <uid>`,
			ActionPrograms:    []string{UnmountStuckBin},
			ActionCategories:  []string{category.SystemDisk},
			ActionProcessHang: true,
		},
	}
}

func (*UnmountStuckActionSpec) Name() string {
	return "unmount_stuck"
}

func (*UnmountStuckActionSpec) Aliases() []string {
	return []string{}
}

func (*UnmountStuckActionSpec) ShortDesc() string {
	return "Simulate volume unmount stuck by holding file handles"
}

func (u *UnmountStuckActionSpec) LongDesc() string {
	if u.ActionLongDesc != "" {
		return u.ActionLongDesc
	}
	return "Simulate volume unmount stuck by holding file handles on the specified mount point, making umount fail with device busy"
}

type UnmountStuckExecutor struct {
	channel spec.Channel
}

func (*UnmountStuckExecutor) Name() string {
	return "unmount_stuck"
}

func (use *UnmountStuckExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	directory := model.ActionFlags["path"]
	if directory == "" {
		log.Errorf(ctx, "path is required")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "path")
	}

	if _, ok := spec.IsDestroy(ctx); ok {
		return use.stop(ctx, directory)
	}

	if !util.IsDir(directory) {
		log.Errorf(ctx, "`%s`: path is illegal, is not a directory", directory)
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "path", directory, "it must be a directory")
	}

	return use.start(ctx, directory)
}

func (use *UnmountStuckExecutor) start(ctx context.Context, directory string) *spec.Response {
	dataFile := path.Join(directory, unmountStuckFile)
	file, err := os.Create(dataFile)
	if err != nil {
		log.Errorf(ctx, "failed to create sentinel file %s: %v", dataFile, err)
		return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("failed to create sentinel file: %v", err))
	}
	// Hold the file handle open, do NOT close it during normal execution.
	// This prevents umount from succeeding on this mount point.
	log.Infof(ctx, "holding file handle on %s, umount will be blocked", directory)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	signal.Stop(sigCh)
	// Ensure the file is not garbage collected before the signal is received.
	runtime.KeepAlive(file)

	// Best-effort cleanup on graceful shutdown
	if err := file.Close(); err != nil {
		log.Errorf(ctx, "failed to close sentinel file %s on shutdown: %v", dataFile, err)
	}
	if err := os.Remove(dataFile); err != nil && !os.IsNotExist(err) {
		log.Errorf(ctx, "failed to remove sentinel file %s on shutdown: %v", dataFile, err)
	}
	return spec.ReturnSuccess(directory)
}

func (use *UnmountStuckExecutor) stop(ctx context.Context, directory string) *spec.Response {
	// Clean up sentinel file using Go native API to avoid command injection
	dataFile := path.Join(directory, unmountStuckFile)
	if _, err := os.Stat(dataFile); err == nil {
		if err := os.Remove(dataFile); err != nil && !os.IsNotExist(err) {
			log.Errorf(ctx, "failed to clean sentinel file %s: %v", dataFile, err)
		}
	}

	ctx = context.WithValue(ctx, "bin", UnmountStuckBin)
	return exec.Destroy(ctx, use.channel, "disk unmount_stuck")
}

func (use *UnmountStuckExecutor) SetChannel(channel spec.Channel) {
	use.channel = channel
}
