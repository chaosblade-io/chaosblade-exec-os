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

package file

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"strings"
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
				NewFileAddActionSpec(),
				NewFileDeleteActionSpec(),
				NewFileMoveActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}

var fileCommFlags = []spec.ExpFlagSpec{
	&spec.ExpFlag{
		Name:     "filepath",
		Desc:     "file path",
		Required: true,
	},
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

func checkFilepathExists(ctx context.Context, cl spec.Channel, filepath string) bool {
	response := cl.Run(ctx, fmt.Sprintf("[ -e %s ] && echo true || echo false", filepath), "")
	fmt.Println(response.Result)
	if response.Success && strings.Contains(response.Result.(string), "true") {
		return true
	}
	return false
}
