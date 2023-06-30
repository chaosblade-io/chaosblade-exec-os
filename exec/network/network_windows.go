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
package network

import (
	"github.com/chaosblade-io/chaosblade-exec-os/exec/network/windows"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

func NewNetworkCommandSpec() spec.ExpModelCommandSpec {
	return &NetworkCommandSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				windows.NewWinNetDelayActionSpec(),
				windows.NewDuplicateActionSpec(),
				windows.NewLossActionSpec(),
				windows.NewOccupyActionSpec(),
				windows.NewReorderActionSpec(),
				windows.NewDnsActionSpec(),
				windows.NewDropActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}
