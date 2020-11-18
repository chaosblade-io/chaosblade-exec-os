package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/chenhy97/chaosblade-exec-os/exec"
	"github.com/chenhy97/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

var (
	straceErrorStart, straceErrorStop, straceErrorNohup bool
	pidList string
	syscallName string
	returnValue string
	first, end, step string
)

var straceErrorBin = exec.StraceErrorBin

func main() {
	flag.BoolVar(&straceErrorStart, "start", false, "start fail syscall")
	flag.BoolVar(&straceErrorStop, "stop", false, "stop fail syscall")
	flag.BoolVar(&straceErrorNohup, "nohup", false, "nohup to run fail syscall")
	flag.StringVar(&pidList, "pid", "", "pids of affected processes")
	flag.StringVar(&syscallName, "syscall-name", "", "failed syscall")
	flag.StringVar(&returnValue, "return-value", "", "injected return value")
	flag.StringVar(&first, "first", "", "the first failed syscall")
	flag.StringVar(&end, "end", "", "the last failed syscall")
	flag.StringVar(&step, "step", "", "the interval between failed syscall")
	bin.ParseFlagAndInitLog()

	if straceErrorStart {
		startError()
	} else if straceErrorStop {
		if success, errs := stopError(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if straceErrorNohup {
		go errorNohup()

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM)
		for s := range ch {
			switch s {
			case syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, os.Interrupt:
				fmt.Printf("caught interrupt, exit")
				return
			}
		}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func startError() {
	args := fmt.Sprintf("%s --nohup --pid %s --syscall-name %s --return-value %s",
		path.Join(util.GetProgramPath(), straceErrorBin), pidList, syscallName, returnValue)
	if first != "" {
		args = fmt.Sprintf("%s --first %s", args, first)
	}
	if end != "" {
		args = fmt.Sprintf("%s --end %s", args, end)
	}
	if step != "" {
		args = fmt.Sprintf("%s --step %s", args, step)
	}
	args = fmt.Sprintf("%s > /dev/null 2>&1 &", args)
	ctx := context.Background()
	response := cl.Run(ctx, "nohup", args)

	if !response.Success {
		stopError()
		bin.PrintErrAndExit(response.Err)
	}
}

func stopError() (success bool, errs string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey, "nohup")
	pids, _ := cl.GetPidsByProcessName(straceErrorBin, ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}
	response := cl.Run(ctx, "kill", fmt.Sprintf(`-HUP %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}
	return true, errs
}

func errorNohup() {
	if pidList != "" {
		pids := strings.Split(pidList, ",")
		args := fmt.Sprintf("-f -e inject=%s:error=%s", syscallName, returnValue)

		if first != "" {
			args = fmt.Sprintf("%s:when=%s", args, first)
			if step != "" && end != "" {
				args = fmt.Sprintf("%s..%s+%s", args, end, step)
			} else if step != "" {
				args = fmt.Sprintf("%s+%s", args, step)
			} else if end != "" {
				args = fmt.Sprintf("%s..%s", args, end)
			}
		}

		for _, pid := range pids {
			args = fmt.Sprintf("-p %s %s", pid, args)
		}

		ctx := context.Background()
		response := cl.Run(ctx, path.Join(util.GetProgramPath(), "strace"), args)

		if !response.Success {
			bin.PrintErrAndExit(response.Err)
		}
		bin.PrintOutputAndExit(response.Result.(string))
		return
	}
}

