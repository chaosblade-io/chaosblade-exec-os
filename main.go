package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
)

const (
	UID     = "uid"
	DEBUG   = "debug"
	CREATE  = "create"
	DESTROY = "destroy"
)

var executors = model.GetAllOsExecutors()
var models = model.GetAllExpModels()
var modelMap = make(map[string]spec.ExpModelCommandSpec)
var modelActionFlags = make(map[string][]spec.ExpFlag)

var uidFlag = spec.ExpFlag{
	Name:    "uid",
	Desc:    "uid",
	Default: "",
}

var debugFlag = spec.ExpFlag{
	Name:    "debug",
	Desc:    "debug",
	Default: "",
}

var channelFlag = spec.ExpFlag{
	Name:    "channel",
	Desc:    "channel",
	Default: "local",
}

var nsTarget = spec.ExpFlag{
	Name:    "ns_target",
	Desc:    "target pid",
	Default: "local",
}

var nsPid = spec.ExpFlag{
	Name:    "ns_pid",
	Desc:    "pid namespace",
	Default: "false",
}

var nsMnt = spec.ExpFlag{
	Name:    "ns_mnt",
	Desc:    "mnt namespace",
	Default: "false",
}

func init() {
	for _, commandSpec := range models {
		modelMap[commandSpec.Name()] = commandSpec
		xes := make([]spec.ExpFlag, len(commandSpec.Flags()))
		for i, f := range commandSpec.Flags() {
			xes[i] = spec.ExpFlag{
				Name:    f.FlagName(),
				Default: f.FlagDefault(),
				Desc:    f.FlagDesc(),
			}
		}
		for _, modelAction := range commandSpec.Actions() {
			matchers := make([]spec.ExpFlag, len(modelAction.Matchers()))
			for i, f := range modelAction.Matchers() {
				matchers[i] = spec.ExpFlag{
					Name:    f.FlagName(),
					Default: f.FlagDefault(),
					Desc:    f.FlagDesc(),
				}
			}
			flags := make([]spec.ExpFlag, len(modelAction.Flags()))
			for i, f := range modelAction.Flags() {
				flags[i] = spec.ExpFlag{
					Name:    f.FlagName(),
					Default: f.FlagDefault(),
					Desc:    f.FlagDesc(),
				}
			}
			util.MergeModels()
			modelActionFlags[commandSpec.Name()+modelAction.Name()] = append(
				append(flags, append(xes, matchers...)...),
				uidFlag,
				channelFlag,
				nsTarget,
				nsPid,
				nsMnt,
				debugFlag,
			)
		}
	}
}

func main() {
	args := os.Args
	if len(args) < 4 {
		fmt.Fprint(os.Stderr, fmt.Sprintf("invalid parameter, %v", args))
		os.Exit(1)
	} else {
		// example => create cpu load cpu-percent=60
		mode := args[1]
		target := args[2]
		action := args[3]

		// get expModel
		expModel := &spec.ExpModel{
			Target:     target,
			ActionName: action,
			ActionFlags: func() map[string]string {
				flagsx := modelActionFlags[target+action]

				flagsValues := make(map[string]*string, len(flagsx))

				cmd := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

				for _, f := range flagsx {
					s := cmd.String(f.Name, f.Default, f.Desc)
					flagsValues[f.Name] = s
				}

				if err := cmd.Parse(os.Args[4:]); err != nil {
					fmt.Fprint(os.Stderr, fmt.Sprintf("invalid parameter, %v", err))
					os.Exit(1)
				}

				actionFlags := make(map[string]string, len(flagsx))
				for k, v := range flagsValues {
					actionFlags[k] = *v
				}
				return actionFlags
			}(),
		}

		ctx := context.Background()
		if mode != CREATE && mode != DESTROY {
			fmt.Fprint(os.Stderr, fmt.Sprintf("invalid parameter, %v", args))
			os.Exit(1)
		}

		uid := expModel.ActionFlags[UID]
		if uid == "" {
			uid, _ = util.GenerateUid()
		}

		if mode == DESTROY {
			ctx = spec.SetDestroyFlag(context.Background(), uid)
		}

		if expModel.ActionFlags[DEBUG] == "true" {
			util.Debug = true
		}
		util.InitLog(util.Bin)
		logrus.Infof("mode: %s, target: %s, action: %s, flags %v", mode, target, action, expModel.ActionFlags)

		key := expModel.Target + expModel.ActionName
		executor := executors[key]
		if executor == nil {
			fmt.Fprint(os.Stderr, fmt.Sprintf("not found executor, target: %s, action: %s", target, action))
			os.Exit(1)
		} else {
			if expModel.ActionFlags["channel"] == "local" {
				executor.SetChannel(channel.NewLocalChannel())
			} else if expModel.ActionFlags["channel"] == "nsexec" {

				ctx = context.WithValue(ctx, "ns_target", expModel.ActionFlags["ns_target"])

				if expModel.ActionFlags["ns_pid"] == "true" {
					ctx = context.WithValue(ctx, "ns_pid", "true")
				}
				if expModel.ActionFlags["ns_mnt"] == "true" {
					ctx = context.WithValue(ctx, "ns_mnt", "true")
				}

				executor.SetChannel(channel.NewNsexecChannel())
			} else {
				executor.SetChannel(channel.NewLocalChannel())
			}

			response := executor.Exec(uid, ctx, expModel)
			logrus.Debugf("os response: %v", response)
			if response.Success {
				fmt.Fprint(os.Stdout, response.Print())
			} else {
				fmt.Fprint(os.Stderr, response.Print())
			}
		}
	}
}
