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

package exec

import (
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type ProcessCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewProcessCommandModelSpec() spec.ExpModelCommandSpec {
	return &ProcessCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpFlags: []spec.ExpFlagSpec{},
			ExpActions: []spec.ExpActionCommandSpec{
				NewKillProcessActionCommandSpec(),
				NewStopProcessActionCommandSpec(),
			},
		},
	}
}

func (*ProcessCommandModelSpec) Name() string {
	return "process"
}

func (*ProcessCommandModelSpec) ShortDesc() string {
	return "Process experiment"
}

func (*ProcessCommandModelSpec) LongDesc() string {
	return "Process experiment, for example, kill process"
}

func (*ProcessCommandModelSpec) Example() string {
	return "blade create process kill --process tomcat"
}

func checkProcessExpEnv() error {
	commands := []string{"ps", "kill", "grep", "tr", "awk"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
