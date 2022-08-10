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

const SecurityGroupBin = "chaos_aliyun_securitygroup"

type SecurityGroupActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewSecurityGroupActionSpec() spec.ExpActionCommandSpec {
	return &SecurityGroupActionSpec{
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
					Desc: "the operation of SecurityGroup, support delete, remove etc",
				},
				&spec.ExpFlag{
					Name: "securityGroupId",
					Desc: "the SecurityGroupId",
				},
			},
			ActionExecutor: &SecurityGroupExecutor{},
			ActionExample: `
# delete securityGroup which securityGroup id is i-x
blade create aliyun securityGroup --accessKeyId xxx --accessKeySecret yyy --regionId cn-qingdao --type delete --securityGroupId i-x

# remove instance i-x from securityGroup which securityGroup id is s-x
blade create aliyun securityGroup --accessKeyId xxx --accessKeySecret yyy --regionId cn-qingdao --type remove --securityGroupId s-x --instanceId i-x

# remove networkInterface n-x from securityGroup which securityGroup id is s-x
blade create aliyun securityGroup --accessKeyId xxx --accessKeySecret yyy --regionId cn-qingdao --type remove --securityGroupId s-x --networkInterfaceId n-x`,
			ActionPrograms:   []string{SecurityGroupBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*SecurityGroupActionSpec) Name() string {
	return "securityGroup"
}

func (*SecurityGroupActionSpec) Aliases() []string {
	return []string{}
}
func (*SecurityGroupActionSpec) ShortDesc() string {
	return "do some aliyun securityGroupId Operations, like delete, remove"
}

func (b *SecurityGroupActionSpec) LongDesc() string {
	if b.ActionLongDesc != "" {
		return b.ActionLongDesc
	}
	return "do some aliyun securityGroupId Operations, like delete, remove"
}

type SecurityGroupExecutor struct {
	channel spec.Channel
}

func (*SecurityGroupExecutor) Name() string {
	return "securityGroup"
}

func (be *SecurityGroupExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if be.channel == nil {
		util.Errorf(uid, util.GetRunFuncName(), spec.ChannelNil.Msg)
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	accessKeyId := model.ActionFlags["accessKeyId"]
	accessKeySecret := model.ActionFlags["accessKeySecret"]
	regionId := model.ActionFlags["regionId"]
	operationType := model.ActionFlags["type"]
	securityGroupId := model.ActionFlags["securityGroupId"]
	instanceId := model.ActionFlags["instanceId"]
	networkInterfaceId := model.ActionFlags["networkInterfaceId"]

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

	if securityGroupId == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "securityGroupId")
	}

	if instanceId != "" && networkInterfaceId != "" {
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "instanceId and networkInterfaceId can not exist both")
	}

	if regionId == "" && networkInterfaceId != "" {
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "networkInterfaceId and instanceId regionId should exist together")
	}

	switch operationType {
	case "delete":
		return deleteSecurityGroup(ctx, accessKeyId, accessKeySecret, regionId, securityGroupId)
	case "remove":
		return removeInstanceFromSecurityGroup(ctx, accessKeyId, accessKeySecret, regionId, securityGroupId, networkInterfaceId, instanceId)
	default:
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "type is not support(support delete, remove)")
	}
	select {}
}

func (be *SecurityGroupExecutor) SetChannel(channel spec.Channel) {
	be.channel = channel
}

// delete securityGroup
func deleteSecurityGroup(ctx context.Context, accessKeyId, accessKeySecret, regionId, securityGroupId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}

	deleteSecurityGroupRequest := &ecs20140526.DeleteSecurityGroupRequest{
		RegionId:        tea.String(regionId),
		SecurityGroupId: tea.String(securityGroupId),
	}
	_, _err = client.DeleteSecurityGroup(deleteSecurityGroupRequest)
	if _err != nil {
		log.Errorf(ctx, "delete aliyun securityGroup failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "delete aliyun securityGroup failed")
	}
	return spec.Success()
}

// remove instance from securityGroup
func removeInstanceFromSecurityGroup(ctx context.Context, accessKeyId, accessKeySecret, regionId, securityGroupId, networkInterfaceId, instanceId string) *spec.Response {
	client, _err := CreateClient(tea.String(accessKeyId), tea.String(accessKeySecret))
	if _err != nil {
		log.Errorf(ctx, "create aliyun client failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "create aliyun client failed")
	}
	if networkInterfaceId != "" {
		leaveSecurityGroupRequest := &ecs20140526.LeaveSecurityGroupRequest{
			SecurityGroupId:    tea.String(securityGroupId),
			RegionId:           tea.String(regionId),
			NetworkInterfaceId: tea.String(networkInterfaceId),
		}
		_, _err = client.LeaveSecurityGroup(leaveSecurityGroupRequest)
	} else {
		leaveSecurityGroupRequest := &ecs20140526.LeaveSecurityGroupRequest{
			SecurityGroupId: tea.String(securityGroupId),
			InstanceId:      tea.String(instanceId),
		}
		_, _err = client.LeaveSecurityGroup(leaveSecurityGroupRequest)
	}
	if _err != nil {
		log.Errorf(ctx, "remove aliyun securityGroup failed, err: %s", _err.Error())
		return spec.ResponseFailWithFlags(spec.ContainerInContextNotFound, "remove aliyun securityGroup failed")
	}
	return spec.Success()
}
