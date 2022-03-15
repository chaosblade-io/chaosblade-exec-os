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

package model

import (
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
)

// Support for other project about chaosblade
func GetAllOsExecutors() map[string]spec.Executor {
	executors := make(map[string]spec.Executor, 0)
	expModels := GetAllExpModels()
	for _, expModel := range expModels {
		executorMap := ExtractExecutorFromExpModel(expModel)
		for key, value := range executorMap {
			executors[key] = value
		}
		expFlagSpecs := append(expModel.Flags(), GetSSHExpFlags()...)
		expModel.SetFlags(expFlagSpecs)
	}
	return executors
}

func GetSHHExecutor() spec.Executor {
	return exec.NewSSHExecutor()
}

func ExtractExecutorFromExpModel(expModel spec.ExpModelCommandSpec) map[string]spec.Executor {
	executors := make(map[string]spec.Executor)
	for _, actionModel := range expModel.Actions() {
		executors[expModel.Name()+actionModel.Name()] = actionModel.Executor()
	}
	return executors
}

func GetSSHExpFlags() []spec.ExpFlagSpec {
	flags := []spec.ExpFlagSpec{
		exec.ChannelFlag,
		exec.SSHHostFlag,
		exec.SSHPortFlag,
		exec.SSHUserFlag,
		exec.SSHKeyFlag,
		exec.SSHKeyPassphraseFlag,
		exec.BladeRelease,
		exec.OverrideBladeRelease,
		exec.InstallPath,
	}
	return flags
}

var UidFlag = spec.ExpFlag{
	Name:    "uid",
	Desc:    "uid",
	Default: "",
}

var DebugFlag = spec.ExpFlag{
	Name:    "debug",
	Desc:    "debug",
	Default: "",
}

var ChannelFlag = spec.ExpFlag{
	Name:    "channel",
	Desc:    "channel",
	Default: "local",
}

var NsTargetFlag = spec.ExpFlag{
	Name:    channel.NSTargetFlagName,
	Desc:    "target pid",
	Default: "",
}

var NsPidFlag = spec.ExpFlag{
	Name:    channel.NSPidFlagName,
	Desc:    "pid namespace",
	Default: "false",
}

var NsMntFlag = spec.ExpFlag{
	Name:    channel.NSMntFlagName,
	Desc:    "mnt namespace",
	Default: "false",
}

var NsNetFlag = spec.ExpFlag{
	Name:    channel.NSNetFlagName,
	Desc:    "net namespace",
	Default: "false",
}
