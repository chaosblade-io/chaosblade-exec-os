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
	"log"
	"os"
	"runtime"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/cpu"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/disk"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/file"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/kernel"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/mem"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/process"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/script"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/systemd"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

// main creates the yaml file of the experiments in the project
func main() {
	if len(os.Args) != 2 {
		log.Panicln("less yaml file path")
	}
	err := util.CreateYamlFile(getModels(), os.Args[1])
	if err != nil {
		log.Panicf("create yaml file error, %v", err)
	}
}

// getModels returns experiment models in the project
func getModels() *spec.Models {

	var modelCommandSpecs []spec.ExpModelCommandSpec

	if runtime.GOOS == spec.OS_WINDOWS {
		modelCommandSpecs = []spec.ExpModelCommandSpec{
			cpu.NewCpuCommandModelSpec(),
			network.NewNetworkCommandSpec(),
			mem.NewMemCommandModelSpec(),
			process.NewProcessCommandModelSpec(),
			file.NewFileCommandSpec(),
			disk.NewDiskCommandSpec(),
		}
	} else if runtime.GOOS == spec.OS_DARWIN {
		modelCommandSpecs = []spec.ExpModelCommandSpec{
			cpu.NewCpuCommandModelSpec(),
			mem.NewMemCommandModelSpec(),
			process.NewProcessCommandModelSpec(),
			network.NewNetworkCommandSpec(),
			disk.NewDiskCommandSpec(),
			script.NewScriptCommandModelSpec(),
			file.NewFileCommandSpec(),
		}
	} else {
		modelCommandSpecs = []spec.ExpModelCommandSpec{
			cpu.NewCpuCommandModelSpec(),
			mem.NewMemCommandModelSpec(),
			process.NewProcessCommandModelSpec(),
			network.NewNetworkCommandSpec(),
			disk.NewDiskCommandSpec(),
			script.NewScriptCommandModelSpec(),
			file.NewFileCommandSpec(),
			kernel.NewKernelInjectCommandSpec(),
			systemd.NewSystemdCommandModelSpec(),
			stressng.NewStressModelSpec(),
		}
	}

	specModels := make([]*spec.Models, 0)
	for _, modeSpec := range modelCommandSpecs {
		flagSpecs := append(modeSpec.Flags(), model.GetSSHExpFlags()...)
		modeSpec.SetFlags(flagSpecs)
		specModel := util.ConvertSpecToModels(modeSpec, spec.ExpPrepareModel{}, "host")
		specModels = append(specModels, specModel)
	}
	return util.MergeModels(specModels...)
}
