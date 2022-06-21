package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"os"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
)

var executors = model.GetAllOsExecutors()
var models = model.GetAllExpModels()
var modelMap = make(map[string]spec.ExpModelCommandSpec)
var modelActionFlags = make(map[string][]spec.ExpFlag)

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
				model.UidFlag,
				model.ChannelFlag,
				model.NsTargetFlag,
				model.NsPidFlag,
				model.NsMntFlag,
				model.NsNetFlag,
				model.DebugFlag,
			)
		}
	}
}

func main() {
	args := os.Args
	if len(args) < 4 {
		exitAndPrint(spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("invalid parameter, %v", args)), 0)
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
					exitAndPrint(spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("invalid parameter, %v", err)), 0)
				}

				actionFlags := make(map[string]string, len(flagsx))
				for k, v := range flagsValues {
					actionFlags[k] = *v
				}
				return actionFlags
			}(),
		}

		ctx := context.Background()
		if mode != spec.Create && mode != spec.Destroy {
			exitAndPrint(spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("invalid parameter, %v", args)), 0)
		}

		uid := expModel.ActionFlags[model.UidFlag.Name]

		ctx = context.WithValue(ctx, spec.Uid, uid)
		if mode == spec.Destroy {
			ctx = spec.SetDestroyFlag(ctx, uid)
		} else {
			if uid == "" {
				uid, _ = util.GenerateUid()
			}
		}

		if expModel.ActionFlags[model.DebugFlag.Name] == spec.True {
			util.Debug = true
		}
		util.InitLog(util.Bin)
		log.Infof(ctx, "mode: %s, target: %s, action: %s, flags %v", mode, target, action, expModel.ActionFlags)

		key := expModel.Target + expModel.ActionName
		executor := executors[key]
		if executor == nil {
			exitAndPrint(spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("not found executor, target: %s, action: %s", target, action)), 0)
		} else {
			if expModel.ActionFlags[model.ChannelFlag.Name] == spec.LocalChannel {
				executor.SetChannel(channel.NewLocalChannel())
			} else if expModel.ActionFlags[model.ChannelFlag.Name] == spec.NSExecBin {

				ctx = context.WithValue(ctx, model.NsTargetFlag.Name, expModel.ActionFlags[model.NsTargetFlag.Name])

				if expModel.ActionFlags[model.NsPidFlag.Name] == spec.True {
					ctx = context.WithValue(ctx, model.NsPidFlag.Name, spec.True)
				}
				if expModel.ActionFlags[model.NsMntFlag.Name] == spec.True {
					ctx = context.WithValue(ctx, model.NsMntFlag.Name, spec.True)
				}
				if expModel.ActionFlags[model.NsNetFlag.Name] == spec.True {
					ctx = context.WithValue(ctx, model.NsNetFlag.Name, spec.True)
				}

				executor.SetChannel(channel.NewNSExecChannel())
			} else {
				executor.SetChannel(channel.NewLocalChannel())
			}
			exitAndPrint(executor.Exec(uid, ctx, expModel), 0)
		}
	}
}

func exitAndPrint(response *spec.Response, code int) {
	fmt.Println(response.Print())
	os.Exit(code)
}
