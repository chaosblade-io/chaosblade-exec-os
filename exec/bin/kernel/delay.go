package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/chenhy97/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/chenhy97/chaosblade-exec-os/exec/bin"
)

var (
	straceDelayStart, straceDelayStop, straceDelayNohup bool
	debug bool
	pidList string
	time string
	syscallName string
	delayLoc string
	first, end, step string
)

var straceDelayBin = exec.StraceDelayBin

func main() {
	flag.BoolVar(&straceDelayStart, "start", false, "start delay syscall")
	flag.BoolVar(&straceDelayStop, "stop", false, "stop delay syscall")
	flag.BoolVar(&straceDelayNohup, "nohup", false, "nohup to run delay syscall")
	flag.StringVar(&pidList, "pid", "", "pids of affected processes")
	flag.StringVar(&time, "time", "", "duration of delay")
	flag.StringVar(&syscallName, "syscall-name", "", "delayed syscall")
	flag.StringVar(&delayLoc, "delay-loc", "enter", "delay position")
	flag.StringVar(&first, "first", "", "the first delayed syscall")
	flag.StringVar(&end, "end", "", "the last delayed syscall")
	flag.StringVar(&step, "step", "", "the interval between delayed syscall")
	bin.ParseFlagAndInitLog()

	if straceDelayStart {
		startDelay()
	} else if straceDelayStop {
		if success, errs := stopDelay(); !success {
			bin.PrintErrAndExit(errs)
		}
	} else if straceDelayNohup {
		go delayNohup()

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL)
		for s := range ch {
			switch s {
			case syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt:
				fmt.Printf("caught interrupt, exit")
				return
			}
		}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func delayNohup() {
	if pidList != "" {
		pids := strings.Split(pidList, ",")

		var args = ""
		if delayLoc == "enter" {
			args = fmt.Sprintf("-f -e inject=%s:delay_enter=%s", syscallName, time)
		} else if delayLoc == "exit" {
			args = fmt.Sprintf("-f -e inject=%s:delay_exit=%s", syscallName, time)
		}

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

func startDelay() {
	args := fmt.Sprintf("%s --nohup --pid %s --time %s --syscall-name %s --delay-loc %s",
		path.Join(util.GetProgramPath(), straceDelayBin), pidList, time, syscallName, delayLoc)

	if first != "" {
		 args = fmt.Sprintf("%s --first %s", args, first)
	}
	if end != "" {
		args = fmt.Sprintf("%s --end %s", args, end)
	}
	if step != "" {
		args = fmt.Sprintf("%s --step %s", args, step)
	}

	args = fmt.Sprintf("%s > debug.log 2>&1 &", args)
	// args = fmt.Sprintf("%s > /dev/null 2>&1 &", args)

	ctx := context.Background()
	response := cl.Run(ctx, "nohup", args)

	if !response.Success {
		stopDelay()
		bin.PrintErrAndExit(response.Err)
	}
}

func stopDelay() (success bool, errs string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey, "nohup")
	pids, _ := cl.GetPidsByProcessName(straceDelayBin, ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}
	response := cl.Run(ctx, "kill", fmt.Sprintf(`-HUP %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}
	return true, errs
}
