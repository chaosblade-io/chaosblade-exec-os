// /*
//  * Copyright 1999-2020 Alibaba Group Holding Ltd.
//  *
//  * Licensed under the Apache License, Version 2.0 (the "License");
//  * you may not use this file except in compliance with the License.
//  * You may obtain a copy of the License at
//  *
//  *     http://www.apache.org/licenses/LICENSE-2.0
//  *
//  * Unless required by applicable law or agreed to in writing, software
//  * distributed under the License is distributed on an "AS IS" BASIS,
//  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  * See the License for the specific language governing permissions and
//  * limitations under the License.
//  */

package exec

import (
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)


type KernelInjectCommandSpec struct{
	spec.BaseExpModelCommandSpec
}

func NewKernelInjectCommandSpec() spec.ExpModelCommandSpec {
	return &KernelInjectCommandSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				NewStraceDelayActionSpec(),
				NewStraceErrorActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}
func (*KernelInjectCommandSpec) Name() string {
	return "strace"
}

func (*KernelInjectCommandSpec) ShortDesc() string {
	return "strace experiment"
}

func (*KernelInjectCommandSpec) LongDesc() string {
	return "strace experiment contains syscall delay or syscall error"
}


func checkKernelInjectExpEnv() error {
	commands := []string{"strace"}
	for _, command := range commands{
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}

