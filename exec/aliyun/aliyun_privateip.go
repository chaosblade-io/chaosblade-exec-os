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
	"strings"
)

const PrivateIpBin = "chaos_aliyun_privateip"

type PrivateIpActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewPrivateIpActionSpec() spec.ExpActionCommandSpec {
	return &PrivateIpActionSpec{
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
					Desc: "the operation of PrivateIp, support unassign etc",
				},
				&spec.ExpFlag{
					Name: "networkInterfaceId",
					Desc: "the networkInterfaceId",
				},
				&spec.ExpFlag{
					Name: "regionId",
					Desc: "the regionId of aliyun",
				},
				&spec.ExpFlag{
					Name: "privateIpAddress",
					Desc: "the PrivateIpAddress",
				},
			},
			ActionExecutor: &PrivateIpExecutor{},
			ActionExample: `
# unassociate private ip from networkInterfaceId n-x which privateIpAddress is 1.1.1.1,2.2.2.2
blade create aliyun privateIp --accessKeyId xxx --accessKeySecret yyy --type unassign --regionId cn-qingdao --networkInterfaceId n-x --privateIpAddress 1.1.1.1,2.2.2.2`,
			ActionPrograms:   []string{PrivateIpBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*PrivateIpActionSpec) Name() string {
	return "privateIp"
}

func (*PrivateIpActionSpec) Aliases() []string {
	return []string{}
}
func (*PrivateIpActionSpec) ShortDesc() string {
	return "do some aliyun private ip Operations, like unassign"
}

func (b *PrivateIpActionSpec) LongDesc() string {
	if b.ActionLongDesc != "" {
		return b.ActionLongDesc
	}
	return "do some aliyun private ip Operations, like unassign"
}

type PrivateIpExecutor struct {
	channel spec.Channel
}

func (*PrivateIpExecutor) Name() string {
	return "privateIp"
}

func (be *PrivateIpExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if be.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	accessKeyId := model.ActionFlags["accessKeyId"]
	accessKeySecret := model.ActionFlags["accessKeySecret"]
	operationType := model.ActionFlags["type"]
	regionId := model.ActionFlags["regionId"]
	networkInterfaceId := model.ActionFlags["networkInterfaceId"]
	privateIpAddress := model.ActionFlags["privateIpAddress"]

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

	if regionId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "regionId")
	}

	if networkInterfaceId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "networkInterfaceId")
	}

	if privateIpAddress == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "privateIpAddress")
	}

	switch operationType {
	case "unassign":
		return unassignPrivateIpAddress(ctx, accessKeyId, accessKeySecret, regionId, networkInterfaceId, strings.Split(privateIpAddress, ","))
	default:
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "type is not support(support unassign)")
	}
	select {}
}

func (be *PrivateIpExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}

// unassign Private Ip
func unassignPrivateIpAddress(ctx context.Context, accessKeyId, accessKeySecret, regionId, networkInterfaceId string, privateIpAddress []string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}

	unassignPrivateIpAddressesRequest := &ecs20140526.UnassignPrivateIpAddressesRequest{
		RegionId:           tea.String(regionId),
		NetworkInterfaceId: tea.String(networkInterfaceId),
		PrivateIpAddress:   tea.StringSlice(privateIpAddress),
	}
	_, _err = client.UnassignPrivateIpAddresses(unassignPrivateIpAddressesRequest)
	if _err != nil {
		log.Errorf(ctx, "unassign aliyun private Ip failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "unassign aliyun private Ip failed")
	}
	return spec.Success()
}
