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

const NetworkInterfaceBin = "chaos_aliyun_networkinterface"

type NetworkInterfaceActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewNetworkInterfaceActionSpec() spec.ExpActionCommandSpec {
	return &NetworkInterfaceActionSpec{
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
					Name: "regionId",
					Desc: "the regionId of aliyun",
				},
				&spec.ExpFlag{
					Name: "instanceId",
					Desc: "the ecs instanceId",
				},
				&spec.ExpFlag{
					Name: "networkInterfaceId",
					Desc: "the networkInterfaceId of aliyun",
				},
				&spec.ExpFlag{
					Name: "type",
					Desc: "the operation of NetworkInterface, support delete, remove etc",
				},
				&spec.ExpFlag{
					Name: "networkInterfaceId",
					Desc: "the NetworkInterfaceId",
				},
			},
			ActionExecutor: &NetworkInterfaceExecutor{},
			ActionExample: `
# delete networkInterface which networkInterface id is i-x
blade create aliyun networkInterface --accessKeyId xxx --accessKeySecret yyy --regionId cn-qingdao --type delete --networkInterfaceId i-x

# remove instance i-x from networkInterface which networkInterface id is s-x
blade create aliyun networkInterface --accessKeyId xxx --accessKeySecret yyy --regionId cn-qingdao --type detach --networkInterfaceId s-x --instanceId i-x`,
			ActionPrograms:   []string{NetworkInterfaceBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*NetworkInterfaceActionSpec) Name() string {
	return "networkInterface"
}

func (*NetworkInterfaceActionSpec) Aliases() []string {
	return []string{}
}
func (*NetworkInterfaceActionSpec) ShortDesc() string {
	return "do some aliyun networkInterfaceId Operations, like delete, remove"
}

func (b *NetworkInterfaceActionSpec) LongDesc() string {
	if b.ActionLongDesc != "" {
		return b.ActionLongDesc
	}
	return "do some aliyun networkInterfaceId Operations, like delete, remove"
}

type NetworkInterfaceExecutor struct {
	channel spec.Channel
}

func (*NetworkInterfaceExecutor) Name() string {
	return "networkInterface"
}

func (be *NetworkInterfaceExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if be.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	accessKeyId := model.ActionFlags["accessKeyId"]
	accessKeySecret := model.ActionFlags["accessKeySecret"]
	regionId := model.ActionFlags["regionId"]
	operationType := model.ActionFlags["type"]
	networkInterfaceId := model.ActionFlags["networkInterfaceId"]
	instanceId := model.ActionFlags["instanceId"]

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

	if operationType == "delete" && regionId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "regionId")
	}

	if networkInterfaceId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "networkInterfaceId")
	}

	if instanceId != "" && networkInterfaceId != "" {
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "instanceId and networkInterfaceId can not exist both")
	}

	if regionId == "" && networkInterfaceId != "" {
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "networkInterfaceId and instanceId regionId should exist together")
	}

	switch operationType {
	case "delete":
		return deleteNetworkInterface(ctx, accessKeyId, accessKeySecret, regionId, networkInterfaceId)
	case "remove":
		return detachNetworkInterfaceFromInstance(ctx, accessKeyId, accessKeySecret, regionId, networkInterfaceId, instanceId)
	default:
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "type is not support(support delete, remove)")
	}
	select {}
}

func (be *NetworkInterfaceExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}

// delete networkInterface
func deleteNetworkInterface(ctx context.Context, accessKeyId, accessKeySecret, regionId, networkInterfaceId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}

	deleteNetworkInterfaceRequest := &ecs20140526.DeleteNetworkInterfaceRequest{
		RegionId:           tea.String(regionId),
		NetworkInterfaceId: tea.String(networkInterfaceId),
	}
	_, _err = client.DeleteNetworkInterface(deleteNetworkInterfaceRequest)

	if _err != nil {
		log.Errorf(ctx, "delete aliyun network interface failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "delete aliyun network interface failed")
	}
	return spec.Success()
}

// remove networkInterface from instance
func detachNetworkInterfaceFromInstance(ctx context.Context, accessKeyId, accessKeySecret, regionId, networkInterfaceId, instanceId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}
	detachNetworkInterfaceRequest := &ecs20140526.DetachNetworkInterfaceRequest{
		RegionId:           tea.String(regionId),
		NetworkInterfaceId: tea.String(networkInterfaceId),
		InstanceId:         tea.String(instanceId),
	}
	_, _err = client.DetachNetworkInterface(detachNetworkInterfaceRequest)
	if _err != nil {
		log.Errorf(ctx, "detach aliyun network interface failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "detach aliyun network interface failed")
	}
	return spec.Success()
}
