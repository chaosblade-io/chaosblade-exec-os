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

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var content, filepath string
var count, interval int
var escape, appendFileStart, appendFileStop, appendFileNoHup bool

func main() {
	flag.StringVar(&content, "content", "", "content")
	flag.StringVar(&filepath, "filepath", "", "filepath")
	flag.IntVar(&count, "count", 1, "append count")
	flag.IntVar(&interval, "interval", 1, "append count")
	flag.BoolVar(&escape, "escape", false, "symbols to escape")
	flag.BoolVar(&appendFileStart, "start", false, "start append file")
	flag.BoolVar(&appendFileStop, "stop", false, "stop append file")
	flag.BoolVar(&appendFileNoHup, "nohup", false, "nohup to run append file")
	bin.ParseFlagAndInitLog()

	if appendFileStart {
		if content == "" || filepath == "" {
			bin.PrintErrAndExit("less --content or --filepath flag")
		}
		startAppendFile(filepath, content, count, interval, escape)
	} else if appendFileNoHup {
		appendFile(filepath, content, count, interval, escape)
		// Wait for signals
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL)
		for s := range ch {
			switch s {
			case syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt:
				fmt.Println("caught interrupt, exit")
				return
			}
		}
	} else if appendFileStop {

		if success, errs := stopAppendFile(filepath); !success {
			bin.PrintErrAndExit(errs)
		}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()
var appendFileBin = "chaos_appendfile"

func startAppendFile(filepath, content string, count int, interval int, escape bool) {
	// check pid
	newCtx := context.WithValue(context.Background(), channel.ProcessKey,
		fmt.Sprintf(`--nohup --filepath %s`, filepath))
	pids, err := cl.GetPidsByProcessName(appendFileBin, newCtx)
	if err != nil {
		stopAppendFile(filepath)
		bin.PrintErrAndExit(fmt.Sprintf("start append file %s failed, cannot get the appending program pid, %v",
			filepath, err))
	}
	if len(pids) > 0 {
		bin.PrintErrAndExit(fmt.Sprintf("start append file %s failed, This file is already being experimented on",
			filepath))
	}

	ctx := context.Background()
	args := fmt.Sprintf(`%s --nohup --filepath "%s" --content "%s" --count %d --interval %d --escape=%t`,
		path.Join(util.GetProgramPath(), appendFileBin), filepath, content, count, interval, escape)
	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := cl.Run(ctx, "nohup", args)
	if !response.Success {
		stopAppendFile(filepath)
		bin.PrintErrAndExit(response.Err)
	}

	// check pid
	newCtx = context.WithValue(context.Background(), channel.ProcessKey,
		fmt.Sprintf(`--nohup --filepath %s`, filepath))
	pids, err = cl.GetPidsByProcessName(appendFileBin, newCtx)
	if err != nil {
		stopAppendFile(filepath)
		bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, cannot get the appending program pid, %v",
			filepath, err))
	}
	if len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, cannot find the appending program pid",
			filepath))
	}
}

func appendFile(filepath string, content string, count int, interval int, escape bool) {

	reg := regexp.MustCompile(`\\?@\{(?s:([^(@{})]*[^\\]))\}|\\?@\[((?s:[^(@{})]*[^\\]))\]|\\?@\w+`)
	result := reg.FindAllStringSubmatch(content, -1)
	for _, text := range result {
		if strings.HasPrefix(text[0], "\\@") {
			content = strings.Replace(content, text[0], strings.Replace(text[0], "\\", "", 1), 1)
			continue
		}
		if text[1] == "" {
			content = strings.Replace(content, text[0], strings.Replace(text[0], "@", "$", 1), 1)
		} else {
			content = strings.Replace(content, text[0], "$("+text[1]+")", 1)
		}
	}

	go func() {
		ctx := context.Background()
		// first append
		if append(count, ctx, content, filepath, escape) {
			return
		}

		ticker := time.NewTicker(time.Second * time.Duration(interval))
		for _ = range ticker.C {
			if append(count, ctx, content, filepath, escape) {
				return
			}
		}
	}()
}

func append(count int, ctx context.Context, content string, filepath string, escape bool) bool {
	var response *spec.Response

	for i := 0; i < count; i++ {
		if escape {
			response = cl.Run(ctx, "echo", fmt.Sprintf(`-e "%s" >> "%s"`, content, filepath))
		} else {
			response = cl.Run(ctx, "echo", fmt.Sprintf(`"%s" >> "%s"`, content, filepath))
		}
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return true
		}
	}
	return false
}

func stopAppendFile(filepath string) (success bool, errs string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey,
		fmt.Sprintf(`--nohup --filepath %s`, filepath))
	pids, _ := cl.GetPidsByProcessName(filepath, ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}

	response := cl.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}
	return true, errs
}
