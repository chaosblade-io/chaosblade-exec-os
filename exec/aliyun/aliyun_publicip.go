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

const PublicIpBin = "chaos_aliyun_publicip"

type PublicIpActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewPublicIpActionSpec() spec.ExpActionCommandSpec {
	return &PublicIpActionSpec{
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
					Desc: "the operation of PublicIp, support release, unassociate, etc",
				},
				&spec.ExpFlag{
					Name: "allocationId",
					Desc: "the allocationId",
				},
				&spec.ExpFlag{
					Name: "regionId",
					Desc: "the regionId of aliyun",
				},
				&spec.ExpFlag{
					Name: "publicIpAddress",
					Desc: "the PublicIpAddress",
				},
			},
			ActionExecutor: &PublicIpExecutor{},
			ActionExample: `
# release publicIp which publicIpAddress is 1.1.1.1
blade create aliyun publicIp --accessKeyId xxx --accessKeySecret yyy --type release --publicIpAddress 1.1.1.1

# unassociate publicIp from instance i-x which allocationId id is a-x
blade create aliyun publicIp --accessKeyId xxx --accessKeySecret yyy --type unassociate --instanceId i-x --allocationId a-x`,
			ActionPrograms:   []string{PublicIpBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*PublicIpActionSpec) Name() string {
	return "publicIp"
}

func (*PublicIpActionSpec) Aliases() []string {
	return []string{}
}
func (*PublicIpActionSpec) ShortDesc() string {
	return "do some aliyun publicIp Operations, like release, unassociate"
}

func (b *PublicIpActionSpec) LongDesc() string {
	if b.ActionLongDesc != "" {
		return b.ActionLongDesc
	}
	return "do some aliyun publicIp Operations, like release, unassociate"
}

type PublicIpExecutor struct {
	channel spec.Channel
}

func (*PublicIpExecutor) Name() string {
	return "publicIp"
}

func (be *PublicIpExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if be.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	accessKeyId := model.ActionFlags["accessKeyId"]
	accessKeySecret := model.ActionFlags["accessKeySecret"]
	operationType := model.ActionFlags["type"]
	regionId := model.ActionFlags["regionId"]
	instanceId := model.ActionFlags["instanceId"]
	allocationId := model.ActionFlags["allocationId"]
	publicIpAddress := model.ActionFlags["publicIpAddress"]

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

	if operationType == "release" && publicIpAddress == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "publicIpAddress")
	}

	if operationType == "unassociate" && allocationId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "allocationId")
	}

	if operationType == "unassociate" && instanceId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "instanceId")
	}

	switch operationType {
	case "release":
		return releasePublicIpAddress(ctx, accessKeyId, accessKeySecret, publicIpAddress, instanceId)
	case "unassociate":
		return unassociateEipAddress(ctx, accessKeyId, accessKeySecret, regionId, allocationId, instanceId)
	default:
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "type is not support(support release, unassociate)")
	}
	select {}
}

func (be *PublicIpExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}

// release Public Ip
func releasePublicIpAddress(ctx context.Context, accessKeyId, accessKeySecret, publicIpAddress, instanceId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}

	if instanceId != "" {
		releasePublicIpAddressRequest := &ecs20140526.ReleasePublicIpAddressRequest{
			PublicIpAddress: tea.String(publicIpAddress),
			InstanceId:      tea.String(instanceId),
		}
		_, _err = client.ReleasePublicIpAddress(releasePublicIpAddressRequest)
	} else {
		releasePublicIpAddressRequest := &ecs20140526.ReleasePublicIpAddressRequest{
			PublicIpAddress: tea.String(publicIpAddress),
		}
		_, _err = client.ReleasePublicIpAddress(releasePublicIpAddressRequest)
	}
	if _err != nil {
		log.Errorf(ctx, "release aliyun public Ip failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "release aliyun public Ip failed")
	}
	return spec.Success()
}

// unassociate Eip Address
func unassociateEipAddress(ctx context.Context, accessKeyId, accessKeySecret, regionId, allocationId, instanceId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}

	if regionId != "" {
		unassociateEipAddressRequest := &ecs20140526.UnassociateEipAddressRequest{
			AllocationId: tea.String(allocationId),
			InstanceId:   tea.String(instanceId),
			RegionId:     tea.String(regionId),
		}
		_, _err = client.UnassociateEipAddress(unassociateEipAddressRequest)
	} else {
		unassociateEipAddressRequest := &ecs20140526.UnassociateEipAddressRequest{
			AllocationId: tea.String(allocationId),
			InstanceId:   tea.String(instanceId),
		}
		_, _err = client.UnassociateEipAddress(unassociateEipAddressRequest)
	}
	if _err != nil {
		log.Errorf(ctx, "unassociate aliyun Eip Address failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "unassociate aliyun Eip Address failed")
	}
	return spec.Success()
}
