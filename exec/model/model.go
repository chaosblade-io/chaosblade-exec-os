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
	"github.com/alecthomas/kong"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

var masters = map[string]WorkMaster{}

// WorkMaster executable spi provider factory
type WorkMaster interface {
	// Assign Runner
	Assign() Worker
}

// Worker executable spi
type Worker interface {
	// Name is the runner provider name
	Name() string
	// Exec parse flags
	Exec() *spec.Response
}

// worker
type Parser struct {
	worker Worker
}

// Name
func (run *Parser) Name() string {
	return run.worker.Name()
}

// Exec
func (run *Parser) Exec() *spec.Response {
	if err := bin.ParseFlagModelAndInitLog(run.worker); nil != err {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], err.Error())
	}
	return run.worker.Exec()
}

// laidOffWorker
type laidOffWorker struct {
	name string
}

// Name
func (run *laidOffWorker) Name() string {
	return run.name
}

// Exec
func (run *laidOffWorker) Exec() *spec.Response {
	return spec.ReturnFail(spec.Code[spec.HandlerNotFound], fmt.Sprintf("Worker %s cant not found ", run.name))
}

// Provide os executor provider with name in init function
func Provide(master WorkMaster) {
	anyone := master.Assign()
	for key, _ := range masters {
		if key == anyone.Name() {
			fmt.Println(fmt.Sprintf("Duplicate spi provider %s has be provide, please check and disable anyone ", anyone.Name()))
			return
		}
	}
	masters[anyone.Name()] = master
}

// Load os executor provider by name
func Load(name string) Worker {
	if master := masters[name]; nil != master {
		return &Parser{worker: master.Assign()}
	}
	return &Parser{worker: &laidOffWorker{name: name}}
}

func GetExpModel(name string, input interface{}) (spec.ExpModelCommandSpec, error) {
	parser, err := kong.New(input)
	if nil != err {
		return nil, err
	}
	model := spec.ExpCommandModel{}
	for _, flag := range parser.Model.Flags {
		model.ExpName = flag.Name
	}
	return &model, nil
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
		exec.NewDiskCommandSpec(),
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
