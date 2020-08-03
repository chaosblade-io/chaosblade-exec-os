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

type FileCommandSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewFileCommandSpec() spec.ExpModelCommandSpec {
	return &FileCommandSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				NewFileAppendActionSpec(),
				NewFileChmodActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}

func (*FileCommandSpec) Name() string {
	return "file"
}

func (*FileCommandSpec) ShortDesc() string {
	return "File experiment"
}

func (*FileCommandSpec) LongDesc() string {
	return "File experiment contains file content append, permission modification so on"
}

func (*FileCommandSpec) Example() string {
	return `blade create file append --filepath /temp/1.log --content "hell world" --count 2 --interval=2`
}

