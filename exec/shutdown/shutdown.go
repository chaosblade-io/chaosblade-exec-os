package shutdown

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"
)

type ShutdownCommandModelSpec struct {
	spec.BaseExpModelCommandSpec
}

var ShutdownCommand = "shutdown"
var stderrLog = "shutdown.err"

// SleepTime Execute shutdown command after 3 seconds
var SleepTime = 3

var Force = spec.ExpFlag{
	Name:                  "force",
	Desc:                  "Force operation",
	NoArgs:                true,
	Required:              false,
	RequiredWhenDestroyed: false,
	Default:               "",
}

var Time = spec.ExpFlag{
	Name:                  "time",
	Desc:                  "waiting time, unit is minute, for example 1 means after 1 minute to run",
	NoArgs:                false,
	Required:              false,
	RequiredWhenDestroyed: false,
	Default:               "now",
}

func NewShutdownCommandModelSpec() spec.ExpModelCommandSpec {
	return &ShutdownCommandModelSpec{
		spec.BaseExpModelCommandSpec{
			ExpActions: []spec.ExpActionCommandSpec{
				NewHaltActionCommandSpec(),
				NewPowerOffActionCommandSpec(),
				NewRebootActionCommandSpec(),
			},
			ExpFlags: []spec.ExpFlagSpec{
				&Time, &Force,
			},
		},
	}
}

func (s ShutdownCommandModelSpec) Name() string {
	return "shutdown"
}

func (s ShutdownCommandModelSpec) ShortDesc() string {
	return "Support shutdown, halt or reboot experiment."
}

func (s ShutdownCommandModelSpec) LongDesc() string {
	return "Support shutdown, halt or reboot experiment. Can control shutdown or restart behavior with different flags. Warning! the experiment cannot be recovered by this tool."
}

func execute(ctx context.Context, model *spec.ExpModel, command string, channel spec.Channel) *spec.Response {
	response := checkShutdownCommand(channel)
	if !response.Success {
		return response
	}
	force := model.ActionFlags[Force.Name] == "true"
	time := model.ActionFlags[Time.Name]
	if time == "" {
		time = "now"
	}
	command = fmt.Sprintf("%s %s", ShutdownCommand, command)
	if force {
		command = fmt.Sprintf("%s -f", command)
	}
	command = fmt.Sprintf("sleep %d && %s %s", SleepTime, command, time)
	shutdownErrLog := util.GetNohupOutput(util.Bin, stderrLog)
	//  nohup bash -c "sleep 3 && shutdown -k" < /dev/null >/dev/null 2> shutdown.err &
	command = fmt.Sprintf("bash -c '%s' < /dev/null > /dev/null 2> %s", command, shutdownErrLog)
	return channel.Run(ctx, "nohup", command)
}

// Cancel shutdown
func cancel(ctx context.Context, uid string, model *spec.ExpModel, channel spec.Channel) *spec.Response {
	time := model.ActionFlags[Time.Name]
	if time == "" || time == "now" || time == "+0" {
		return spec.ReturnSuccess(uid)
	}
	// Calling the cancel command directly will not process the execution result.
	// Because the return may fail due to downtime, it returns success directly.
	response := channel.Run(ctx, ShutdownCommand, "-c")
	if !response.Success {
		logrus.Warningf("uid: %s, shutdown cancel failed, %v", uid, response.Error())
	}
	// Not bug.
	return spec.ReturnSuccess(uid)
}

// checkShutdownCommand use shutdown -c command to test
func checkShutdownCommand(channel spec.Channel) *spec.Response {
	if !channel.IsCommandAvailable(context.Background(), ShutdownCommand) {
		return spec.Return(spec.CommandShutdownNotFound, false)
	}
	return channel.Run(context.Background(), ShutdownCommand, "-c")
}
