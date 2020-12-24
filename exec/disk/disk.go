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

package disk

import (
	"fmt"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

var StartFlag = spec.ExpFlag{
	Name:   "start",
	Desc:   "Start chaos experiment",
	NoArgs: true,
}

var StopFlag = spec.ExpFlag{
	Name:   "stop",
	Desc:   "Stop chaos experiment",
	NoArgs: true,
}

var NohupFlag = spec.ExpFlag{
	Name:   "nohup",
	Desc:   "Use nohup command to execute other command. Don't add this flag manually.",
	NoArgs: true,
}

var ReadFlag = spec.ExpFlag{
	Name:   "read",
	Desc:   "Burn io by read, it will create a 600M for reading and delete it when destroy it",
	NoArgs: true,
}

var WriteFlag = spec.ExpFlag{
	Name:   "write",
	Desc:   "Burn io by write, it will create a file by value of the size flag, for example the size default value is 10, then it will create a 10M*100=1000M file for writing, and delete it when destroy",
	NoArgs: true,
}

var SizeFlag = spec.ExpFlag{
	Name: "size",
	Desc: "Block size, MB, default is 10",
}

var PathFlag = spec.ExpFlag{
	Name: "path",
	Desc: "The path of directory where the disk is burning, default value is /",
}

type CommandSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewDiskCommandSpec() spec.ExpModelCommandSpec {
	return &CommandSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				NewFillActionSpec(),
				NewBurnActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}

func (*CommandSpec) Name() string {
	return "disk"
}

func (*CommandSpec) ShortDesc() string {
	return "Disk experiment"
}

func (*CommandSpec) LongDesc() string {
	return "Disk experiment contains fill disk or burn io"
}

func CheckDiskExpEnv() error {
	commands := []string{"rm", "dd"}
	for _, command := range commands {
		if !channel.NewLocalChannel().IsCommandAvailable(command) {
			return fmt.Errorf("%s command not found", command)
		}
	}
	return nil
}
