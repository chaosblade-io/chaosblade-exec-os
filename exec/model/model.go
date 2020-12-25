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
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/disk"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

var providers = map[string]Runner{}

// Runner executable spi
type Runner interface {
	// Exec parse flags
	Exec(input ...interface{}) *spec.Response
}

// FnRunner wrap function as Runner
type FnRunner struct {
	Fn func(input ...interface{}) *spec.Response
}

// Runner delegate execute
func (run *FnRunner) Exec(input ...interface{}) *spec.Response {
	return run.Fn(input...)
}

type NopRunner struct {
	name string
}

func (run *NopRunner) Exec(input ...interface{}) *spec.Response {
	return spec.ReturnFail(spec.Code[spec.HandlerNotFound], fmt.Sprintf("Spi provider %s cant not found ", run.name))
}

// Provide os executor provider with name in init function
func Provide(name string, provider Runner) {
	for key, _ := range providers {
		if key == name {
			fmt.Println(fmt.Sprintf("Duplicate spi provider %s has be provide, please check and disable anyone ", name))
			return
		}
	}
	providers[name] = provider
}

// Provide unmutable parameters function
func ProvideFn(name string, exec func(input interface{}) *spec.Response) {
	Provide(name, &FnRunner{Fn: func(input ...interface{}) *spec.Response {
		if len(input) > 0 {
			return exec(input[0])
		} else {
			return exec(nil)
		}
	}})
}

// Load os executor provider by name
func Load(name string) Runner {
	if provider := providers[name]; nil != provider {
		return provider
	}
	return &NopRunner{name: name}
}

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

// GetAllExpModels returns the experiment model specs in the project.
// Support for other project about chaosblade
func GetAllExpModels() []spec.ExpModelCommandSpec {
	return []spec.ExpModelCommandSpec{
		exec.NewCpuCommandModelSpec(),
		exec.NewMemCommandModelSpec(),
		exec.NewProcessCommandModelSpec(),
		exec.NewNetworkCommandSpec(),
		disk.NewDiskCommandSpec(),
		exec.NewScriptCommandModelSpec(),
		exec.NewFileCommandSpec(),
		exec.NewKernelInjectCommandSpec(),
	}
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
