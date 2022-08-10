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

package aliyun

import (
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type AliyunCommandSpec struct {
	spec.BaseExpModelCommandSpec
}

func NewAliyunCommandSpec() spec.ExpModelCommandSpec {
	return &AliyunCommandSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				NewEcsActionSpec(),
				NewVSwitchActionSpec(),
				NewSecurityGroupActionSpec(),
				NewNetworkInterfaceActionSpec(),
				NewPublicIpActionSpec(),
				NewPrivateIpActionSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{},
		},
	}
}

func (*AliyunCommandSpec) Name() string {
	return "aliyun"
}

func (*AliyunCommandSpec) ShortDesc() string {
	return "Aliyun experiment"
}

func (*AliyunCommandSpec) LongDesc() string {
	return "Aliyun experiment contains ecs, public ip, private ip, networkInterface, securityGroup, VSwitch"
}
