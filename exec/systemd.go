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
 
 type SystemdCommandModelSpec struct {
	 spec.BaseExpModelCommandSpec
 }
 
 func NewSystemdCommandModelSpec() spec.ExpModelCommandSpec {
	 return &SystemdCommandModelSpec{
		 spec.BaseExpModelCommandSpec{
			 ExpFlags: []spec.ExpFlagSpec{
				 &spec.ExpFlag{
					 Name:   "ignore-not-found",
					 Desc:   "Ignore systemd that cannot be found",
					 NoArgs: true,
				 },
			 },
			 ExpActions: []spec.ExpActionCommandSpec{
				 NewStopSystemdActionCommandSpec(),
			 },
		 },
	 }
 }
 
 func (*SystemdCommandModelSpec) Name() string {
	 return "systemd"
 }
 
 func (*SystemdCommandModelSpec) ShortDesc() string {
	 return "Systemd experiment"
 }
 
 func (*SystemdCommandModelSpec) LongDesc() string {
	 return "Systemd experiment, for example, stop systemd"
 }
 