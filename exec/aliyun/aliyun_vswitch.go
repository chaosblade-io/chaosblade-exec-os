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
	"context"
	ecs20140526 "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"os"
)

const VSwitchBin = "chaos_aliyun_vswitch"

type VSwitchActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewVSwitchActionSpec() spec.ExpActionCommandSpec {
	return &VSwitchActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "accessKeyId",
					Desc: "the accessKeyId of aliyun, if not provided, get from env ACCESS_KEY_ID",
				},
				&spec.ExpFlag{
					Name: "accessKeySecret",
					Desc: "the accessKeySecret of aliyun, if not provided, get from env ACCESS_KEY_SECRET",
				},
				&spec.ExpFlag{
					Name: "type",
					Desc: "the operation of VSwitch, support delete etc",
				},
				&spec.ExpFlag{
					Name: "vSwitchId",
					Desc: "the VSwitchId",
				},
			},
			ActionExecutor: &VSwitchExecutor{},
			ActionExample: `
# delete vSwitch which vSwitch id is i-x
blade create aliyun vSwitch --accessKeyId xxx --accessKeySecret yyy --type delete --vSwitchId i-x`,
			ActionPrograms:   []string{VSwitchBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*VSwitchActionSpec) Name() string {
	return "vSwitch"
}

func (*VSwitchActionSpec) Aliases() []string {
	return []string{}
}
func (*VSwitchActionSpec) ShortDesc() string {
	return "do some aliyun vSwitchId Operations, like delete"
}

func (b *VSwitchActionSpec) LongDesc() string {
	if b.ActionLongDesc != "" {
		return b.ActionLongDesc
	}
	return "do some aliyun vSwitchId Operations, like delete"
}

type VSwitchExecutor struct {
	channel spec.Channel
}

func (*VSwitchExecutor) Name() string {
	return "vSwitch"
}

func (be *VSwitchExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if be.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	accessKeyId := model.ActionFlags["accessKeyId"]
	accessKeySecret := model.ActionFlags["accessKeySecret"]
	operationType := model.ActionFlags["type"]
	vSwitchId := model.ActionFlags["vSwitchId"]

	if accessKeyId == "" {
		val, ok := os.LookupEnv("ACCESS_KEY_ID")
		if !ok {
			return spec.ResponseFailWithFlags(spec.ParameterLess, "accessKeyId")
		}
		accessKeyId = val
	}

	if accessKeySecret == "" {
		val, ok := os.LookupEnv("ACCESS_KEY_SECRET")
		if !ok {
			return spec.ResponseFailWithFlags(spec.ParameterLess, "accessKeySecret")
		}
		accessKeySecret = val
	}

	if vSwitchId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "vSwitchId")
	}

	switch operationType {
	case "delete":
		return deleteVSwitch(ctx, accessKeyId, accessKeySecret, vSwitchId)
	default:
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "type is not support(support delete, remove)")
	}
	select {}
}

func (be *VSwitchExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}

// delete vSwitch
func deleteVSwitch(ctx context.Context, accessKeyId, accessKeySecret, vSwitchId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}

	deleteVSwitchRequest := &ecs20140526.DeleteVSwitchRequest{
		VSwitchId: tea.String(vSwitchId),
	}

	_, _err = client.DeleteVSwitch(deleteVSwitchRequest)
	if _err != nil {
		log.Errorf(ctx, "delete aliyun vSwitch failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "delete aliyun vSwitch failed")
	}
	return spec.Success()
}
