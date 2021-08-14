package exec

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type IOHangActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewIOHangActionSpec() spec.ExpActionCommandSpec {
	return &IOHangActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:   "read",
					Desc:   "Read type to take effect",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "write",
					Desc:   "Write type to take effect",
					NoArgs: true,
				},
			},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "device",
					Desc:     "Blocked device used blade query disk lsblk command to get",
					Required: false,
				},
				&spec.ExpFlag{
					Name:     "devices",
					Desc:     "Blocked devices exclude system device, multiple devices separated by commas",
					Required: false,
				},
			},
			ActionExecutor: &IOHangActionExecutor{},
		},
	}
}

func (*IOHangActionSpec) Name() string {
	return "iohang"
}

func (*IOHangActionSpec) Aliases() []string {
	return []string{}
}

func (*IOHangActionSpec) ShortDesc() string {
	return "Block all IO on a device"
}

func (*IOHangActionSpec) LongDesc() string {
	return "Block all IO on a device"
}

type IOHangActionExecutor struct {
	channel spec.Channel
}

func (*IOHangActionExecutor) Name() string {
	return "iohang"
}

func (ioae *IOHangActionExecutor) SetChannel(channel spec.Channel) {
	ioae.channel = channel
}

func (ioae *IOHangActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if ioae.channel == nil {
		return spec.ResponseFailWithFlags(spec.ChannelNil)
	}
	device := model.ActionFlags["device"]
	devicesStr := model.ActionFlags["devices"]
	if device == "" && devicesStr == "" {
		return spec.ResponseFailWithFlags(spec.ParameterLess, "device|devices")
	}
	if device != "" && devicesStr != "" {
		return spec.ResponseFailWithFlags(spec.ParameterIllegal, "device|devices", fmt.Sprintf("%s|%s", device, devicesStr),
			"only select one between --device and --devices arguments")
	}
	var devices []string
	if devicesStr != "" {
		devices = strings.Split(devicesStr, ",")
	}
	if device != "" {
		devices = append(devices, device)
	}
	read := model.ActionFlags["read"] == "true"
	write := model.ActionFlags["write"] == "true"
	if _, ok := spec.IsDestroy(ctx); ok {
		return ioae.stop(devices, read, write, ctx)
	} else {
		return ioae.start(devices, read, write, ctx)
	}
}

const iohangScriptName = "disk_controller.py"

func (ioae *IOHangActionExecutor) start(devices []string, read, write bool, ctx context.Context) *spec.Response {
	// python disk_controller.py -d device -t rw -a hang
	var successDevices []string
	var errorMessages []string
	for _, device := range devices {
		if device == "" {
			continue
		}
		args := fmt.Sprintf("%s -d %s -a hang -t %s",
			path.Join(ioae.channel.GetScriptPath(), iohangScriptName), device, getTypeArg(read, write))
		response := ioae.channel.Run(ctx, "python", args)
		if response.Success {
			successDevices = append(successDevices, device)
		} else {
			errorMessages = append(errorMessages, response.Err)
		}
	}
	if len(successDevices) == 0 {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, iohangScriptName, strings.Join(errorMessages, "|"))
	}
	// 如果执行成功，则立即返回，不需要去写本地数据库
	response := spec.Return(spec.ReturnOKDirectly, true)
	if len(errorMessages) != 0 {
		response.Err = fmt.Sprintf("success device: %s, other error messages: %s",
			strings.Join(successDevices, ","), strings.Join(errorMessages, "|"))
	} else {
		response.Result = spec.OK.Msg
	}
	return response
}

func (ioae *IOHangActionExecutor) stop(devices []string, read, write bool, ctx context.Context) *spec.Response {
	// python disk_controller.py -d device -t rw -a recover
	var successDevices []string
	var errorMessages []string
	for _, device := range devices {
		if device == "" {
			continue
		}
		args := fmt.Sprintf("%s -d %s -a recover -t %s",
			path.Join(ioae.channel.GetScriptPath(), iohangScriptName), device, getTypeArg(read, write))
		response := ioae.channel.Run(ctx, "python", args)
		if response.Success {
			successDevices = append(successDevices, device)
		} else {
			errorMessages = append(errorMessages, response.Err)
		}
	}
	if len(errorMessages) != 0 {
		return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, iohangScriptName,
			fmt.Sprintf("success stoped devices: %s, other error messages: %s",
				strings.Join(successDevices, ","),
				strings.Join(errorMessages, "|")))
	}
	return spec.ReturnSuccess(fmt.Sprintf("success stoped devices: %s", strings.Join(successDevices, ",")))
}

func getTypeArg(read, write bool) string {
	if read && write {
		return "rw"
	} else if read {
		return "read"
	} else if write {
		return "write"
	} else {
		return "rw"
	}
}
