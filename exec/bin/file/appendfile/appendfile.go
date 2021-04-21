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

package appendfile

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/model"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

// init registry provider to model.
func init() {
	model.Provide(new(AppendFile))
}

type AppendFile struct {
	Content         string `name:"content" json:"content" yaml:"content" default:"" help:"content"`
	Filepath        string `name:"filepath" json:"filepath" yaml:"filepath" default:"" help:"filepath"`
	Count           int    `name:"count" json:"count" yaml:"count" default:"1" help:"append count"`
	Interval        int    `name:"interval" json:"interval" yaml:"interval" default:"1" help:"append interval"`
	Escape          bool   `name:"escape" json:"escape" yaml:"escape" default:"false" help:"symbols to escape"`
	EnableBase64    bool   `name:"enable-base64" json:"enable-base64" yaml:"enable-base64" default:"false" help:"append content enableBase64 encoding"`
	AppendFileStart bool   `name:"start" json:"start" yaml:"start" default:"false" help:"start append file"`
	AppendFileStop  bool   `name:"stop" json:"stop" yaml:"stop" default:"false" help:"stop append file"`
	AppendFileNoHup bool   `name:"nohup" json:"nohup" yaml:"nohup" default:"false" help:"nohup to run append file"`
	// default arguments
	Channel channel.OsChannel `kong:"-"`
	// for test mock
}

func (that *AppendFile) Assign() model.Worker {
	return &AppendFile{Channel: channel.NewLocalChannel()}
}

func (that *AppendFile) Name() string {
	return exec.AppendFileBin
}

func (that *AppendFile) Exec() *spec.Response {
	if that.AppendFileStart {
		if that.Content == "" || that.Filepath == "" {
			bin.PrintErrAndExit("less --content or --filepath flag")
		}
		if strings.Contains(that.Content, "@@##") {
			that.Content = strings.Replace(that.Content, "@@##", " ", -1)
		}
		that.startAppendFile(that.Filepath, that.Content, that.Count, that.Interval, that.Escape, that.EnableBase64)
	} else if that.AppendFileNoHup {
		that.appendFile(that.Filepath, that.Content, that.Count, that.Interval, that.Escape, that.EnableBase64)
		// Wait for signals
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL)
		for s := range ch {
			switch s {
			case syscall.SIGHUP, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt:
				fmt.Println("caught interrupt, exit")
				return spec.ReturnSuccess("")
			}
		}
	} else if that.AppendFileStop {

		if success, errs := that.stopAppendFile(that.Filepath); !success {
			bin.PrintErrAndExit(errs)
		}
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
	return spec.ReturnSuccess("")
}

func (that *AppendFile) startAppendFile(filepath, content string, count int, interval int, escape bool, enableBase64 bool) {
	// check pid
	newCtx := context.WithValue(context.Background(), channel.ProcessKey,
		fmt.Sprintf(`--nohup --filepath %s`, filepath))
	pids, err := that.Channel.GetPidsByProcessName(that.Name(), newCtx)
	if err != nil {
		that.stopAppendFile(filepath)
		bin.PrintErrAndExit(fmt.Sprintf("start append file %s failed, cannot get the appending program pid, %v",
			filepath, err))
	}
	if len(pids) > 0 {
		bin.PrintErrAndExit(fmt.Sprintf("start append file %s failed, This file is already being experimented on",
			filepath))
	}

	ctx := context.Background()
	args := fmt.Sprintf(`%s --nohup --filepath "%s" --content "%s" --count %d --interval %d --escape=%t --enable-base64=%t`,
		path.Join(util.GetProgramPath(), that.Name()), filepath, content, count, interval, escape, enableBase64)
	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := that.Channel.Run(ctx, "nohup", args)
	if !response.Success {
		that.stopAppendFile(filepath)
		bin.PrintErrAndExit(response.Err)
	}

	// check pid
	newCtx = context.WithValue(context.Background(), channel.ProcessKey,
		fmt.Sprintf(`--nohup --filepath %s`, filepath))
	pids, err = that.Channel.GetPidsByProcessName(that.Name(), newCtx)
	if err != nil {
		that.stopAppendFile(filepath)
		bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, cannot get the appending program pid, %v",
			filepath, err))
	}
	if len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, cannot find the appending program pid",
			filepath))
	}
}

func (that *AppendFile) appendFile(filepath string, content string, count int, interval int, escape bool, enableBase64 bool) {

	go func() {
		ctx := context.Background()
		// first append
		if that.append(count, ctx, content, filepath, escape, enableBase64) {
			return
		}

		ticker := time.NewTicker(time.Second * time.Duration(interval))
		for range ticker.C {
			if that.append(count, ctx, content, filepath, escape, enableBase64) {
				return
			}
		}
	}()
}

func parseDate(content string) string {
	reg := regexp.MustCompile(`\\?@\{(?s:DATE\:([^(@{})]*[^\\]))\}`)
	result := reg.FindAllStringSubmatch(content, -1)
	for _, text := range result {
		if strings.HasPrefix(text[0], "\\@") {
			content = strings.Replace(content, text[0], strings.Replace(text[0], "\\", "", 1), 1)
			continue
		}
		content = strings.Replace(content, text[0], "$(date \""+text[1]+"\")", 1)
	}
	return content
}

func parseRandom(content string) string {
	reg := regexp.MustCompile(`\\?@\{(?s:RANDOM\:([0-9]+\-[0-9]+))\}`)
	result := reg.FindAllStringSubmatch(content, -1)
	for _, text := range result {
		if strings.HasPrefix(text[0], "\\@") {
			content = strings.Replace(content, text[0], strings.Replace(text[0], "\\", "", 1), 1)
			continue
		}
		split := strings.Split(text[1], "-")
		begin, err := strconv.Atoi(split[0])
		if err != nil {
			bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, radom expression can not parse", text[1]))
		}

		end, err := strconv.Atoi(split[1])
		if err != nil {
			bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, radom expression can not parse", text[1]))
		}

		if end <= begin {
			bin.PrintErrAndExit(fmt.Sprintf("run append file %s failed, begin must < end", text[1]))
		}
		content = strings.Replace(content, text[0], strconv.Itoa(rand.Intn(end-begin)+begin), 1)
	}
	return content
}

func (that *AppendFile) append(count int, ctx context.Context, content string, filepath string, escape bool, enableBase64 bool) bool {
	var response *spec.Response
	if enableBase64 {
		decodeBytes, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			bin.PrintErrAndExit(err.Error())
			return true
		}
		content = string(decodeBytes)
	}
	content = parseDate(content)
	for i := 0; i < count; i++ {
		content = parseRandom(content)
		if escape {
			response = that.Channel.Run(ctx, "echo", fmt.Sprintf(`-e "%s" >> "%s"`, content, filepath))
		} else {
			response = that.Channel.Run(ctx, "echo", fmt.Sprintf(`"%s" >> "%s"`, content, filepath))
		}
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return true
		}
	}
	return false
}

func (that *AppendFile) stopAppendFile(filepath string) (success bool, errs string) {
	ctx := context.WithValue(context.Background(), channel.ProcessKey,
		fmt.Sprintf(`--nohup --filepath %s`, filepath))
	pids, _ := that.Channel.GetPidsByProcessName(filepath, ctx)
	if pids == nil || len(pids) == 0 {
		return true, errs
	}

	response := that.Channel.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
	if !response.Success {
		return false, response.Err
	}
	return true, errs
}
