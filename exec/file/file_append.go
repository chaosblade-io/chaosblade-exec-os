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

package file

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const AppendFileBin = "chaos_appendfile"

type FileAppendActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileAppendActionSpec() spec.ExpActionCommandSpec {
	return &FileAppendActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "content",
					Desc:     "append content",
					Required: true,
				},
				&spec.ExpFlag{
					Name: "count",
					Desc: "the number of append count, must be a positive integer, default 1",
				},
				&spec.ExpFlag{
					Name: "interval",
					Desc: "append interval, must be a positive integer, default 1s",
				},
				&spec.ExpFlag{
					Name:   "escape",
					Desc:   "symbols to escape, use --escape, at this --count is invalid",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "enable-base64",
					Desc:   "append content enable base64 encoding",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:     "cgroup-root",
					Desc:     "cgroup root path, default value /sys/fs/cgroup",
					NoArgs:   false,
					Required: false,
					Default: "/sys/fs/cgroup",
				},
			},
			ActionExecutor: &FileAppendActionExecutor{},
			ActionExample: `
# Appends the content "HELLO WORLD" to the /home/logs/nginx.log file
blade create file append --filepath=/home/logs/nginx.log --content="HELL WORLD"

# Appends the content "HELLO WORLD" to the /home/logs/nginx.log file, interval 10 seconds
blade create file append --filepath=/home/logs/nginx.log --content="HELL WORLD" --interval 10

# Appends the content "HELLO WORLD" to the /home/logs/nginx.log file, enable base64 encoding
blade create file append --filepath=/home/logs/nginx.log --content=SEVMTE8gV09STEQ=

# mock interface timeout exception
blade create file append --filepath=/home/logs/nginx.log --content="@{DATE:+%Y-%m-%d %H:%M:%S} ERROR invoke getUser timeout [@{RANDOM:100-200}]ms abc  mock exception"
`,
			ActionPrograms:    []string{AppendFileBin},
			ActionCategories:  []string{category.SystemFile},
			ActionProcessHang: true,
		},
	}
}

func (*FileAppendActionSpec) Name() string {
	return "append"
}

func (*FileAppendActionSpec) Aliases() []string {
	return []string{}
}

func (*FileAppendActionSpec) ShortDesc() string {
	return "File content append"
}

func (f *FileAppendActionSpec) LongDesc() string {
	return "File content append. "
}

type FileAppendActionExecutor struct {
	channel spec.Channel
}

func (*FileAppendActionExecutor) Name() string {
	return "append"
}

func (f *FileAppendActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"echo", "kill"}
	if response, ok := f.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	filepath := model.ActionFlags["filepath"]
	if _, ok := spec.IsDestroy(ctx); ok {
		return f.stop(filepath, ctx)
	}

	if !exec.CheckFilepathExists(ctx, f.channel, filepath) {
		log.Errorf(ctx,"`%s`: file does not exist", filepath)
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "filepath", filepath, "the file does not exist")
	}

	// default 1
	count := 1
	// 1000 ms
	interval := 1

	content := model.ActionFlags["content"]
	countStr := model.ActionFlags["count"]
	intervalStr := model.ActionFlags["interval"]
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count < 1 {
			log.Errorf(ctx,"`%s` value must be a positive integer", "count")
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "it must be a positive integer")
		}
	}
	if intervalStr != "" {
		var err error
		interval, err = strconv.Atoi(intervalStr)
		if err != nil || interval < 1 {
			log.Errorf(ctx, "`%s` value must be a positive integer", "interval")
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "interval", interval, "it must be a positive integer")
		}
	}

	escape := model.ActionFlags["escape"] == "true"
	enableBase64 := model.ActionFlags["enable-base64"] == "true"

	return f.start(filepath, content, count, interval, escape, enableBase64, ctx)
}

func (f *FileAppendActionExecutor) start(filepath string, content string, count int, interval int, escape bool, enableBase64 bool, ctx context.Context) *spec.Response {
	// first append
	response := appendFile(f.channel, count, ctx, content, filepath, escape, enableBase64)
	if !response.Success {
		return response
	}

	ticker := time.NewTicker(time.Second * time.Duration(interval))
	for range ticker.C {
		response := appendFile(f.channel, count, ctx, content, filepath, escape, enableBase64)
		if !response.Success {
			return response
		}
	}
	return nil
}

func (f *FileAppendActionExecutor) stop(filepath string, ctx context.Context) *spec.Response {
	ctx = context.WithValue(ctx,"bin", AppendFileBin)
	return exec.Destroy(ctx, f.channel, "file append")
}

func (f *FileAppendActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}

func appendFile(cl spec.Channel, count int, ctx context.Context, content string, filepath string, escape bool, enableBase64 bool) *spec.Response {
	var response *spec.Response
	if enableBase64 {
		decodeBytes, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s base64 decode err", content))
		}
		content = string(decodeBytes)
	}
	content = parseDate(content)
	for i := 0; i < count; i++ {
		response = parseRandom(content)
		if !response.Success {
			return response
		}
		content = response.Result.(string)
		if escape {
			response = cl.Run(ctx, "echo", fmt.Sprintf(`-e '%s' >> %s`, content, filepath))
		} else {
			response = cl.Run(ctx, "echo", fmt.Sprintf(`'%s' >> %s`, content, filepath))
		}
	}
	return response
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

func parseRandom(content string) *spec.Response {
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
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("%s illegal parameter", begin))
		}

		end, err := strconv.Atoi(split[1])
		if err != nil {
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("%s illegal parameter", end))
		}

		if end <= begin {
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("run append file %s failed, begin must < end"))
		}
		content = strings.Replace(content, text[0], strconv.Itoa(rand.Intn(end-begin)+begin), 1)
	}
	return spec.ReturnSuccess(content)
}
