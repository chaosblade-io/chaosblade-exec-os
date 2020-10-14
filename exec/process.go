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
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type ProcessCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewProcessCommandModelSpec() spec.ExpModelCommandSpec {
	return &ProcessCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:   "ignore-not-found",
					Desc:   "Ignore process that cannot be found",
					NoArgs: true,
				},
			},
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
