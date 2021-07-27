package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/sirupsen/logrus"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
)

const (
	SEPARATOR = "="
	UID       = "uid"
	DEBUG     = "debug"
	CREATE    = "create"
	DESTROY   = "destroy"
)

var executors = model.GetAllOsExecutors()

func main() {

	args := os.Args
	if len(args) < 4 {
		bin.PrintErrAndExit(fmt.Sprintf("invalid parameter, %v", args))
	} else {
		mode := args[1]
		target := args[2]
		action := args[3]

		ctx := context.Background()
		if mode != CREATE && mode != DESTROY {
			bin.PrintErrAndExit(fmt.Sprintf("invalid parameter, %v", args))
		}

		flags := args[4:]
		expModel := createExpModel(target, action, flags)

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
		logrus.Infof("mode: %s, target: %s, action: %s, flags %v", mode, target, action, flags)

		key := expModel.Target + expModel.ActionName
		executor := executors[key]
		if executor == nil {
			bin.PrintErrAndExit(fmt.Sprintf("not found executor, target: %s, action: %s", target, action))
		} else {
			executor.SetChannel(channel.NewLocalChannel())
			response := executor.Exec(uid, ctx, expModel)
			logrus.Debugf("os response: %v", response)
			if response.Success {
				bin.PrintOutputAndExit(response.Print())
			}
			bin.PrintErrAndExit(response.Print())
		}
	}
}

func createExpModel(target, actionName string, flags []string) *spec.ExpModel {
	expModel := &spec.ExpModel{
		Target:      target,
		ActionName:  actionName,
		ActionFlags: make(map[string]string, 0),
	}
	for _, flag := range flags {
		split := strings.Split(flag, SEPARATOR)
		if len(split) == 2 {
			expModel.ActionFlags[split[0]] = split[1]
		} else {
			logrus.Errorf("invalid flag, %s", flag)
		}
	}
	return expModel
}
