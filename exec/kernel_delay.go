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

 package exec

 import (
	 "context"
	 "fmt"
	 "path"
	 "strings"

	 "github.com/chaosblade-io/chaosblade-spec-go/spec"
	 "github.com/chaosblade-io/chaosblade-spec-go/util"
 )

 const StraceDelayBin = "chaos_stracedelay"

 type StraceDelayActionSpec struct {
	 spec.BaseExpActionCommandSpec
 }

 func NewStraceDelayActionSpec() spec.ExpActionCommandSpec {
	 return &StraceDelayActionSpec{
		 spec.BaseExpActionCommandSpec{
			 ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "pid",
					Desc: "The Pid of the target process",
					Required: true,
				},
			 },
			 ActionFlags: []spec.ExpFlagSpec{
				 &spec.ExpFlag{
					Name: "syscall-name",
					Desc: "The target syscall which will be injected",
					Required: true,
				 },
				 &spec.ExpFlag{
					Name: "time",
					Desc: "sleep time, the unit of time can be specified: s,ms,us,ns",
					Required: true,
				 },
				 &spec.ExpFlag{
					Name: "delay-loc",
					Desc: "if the flag is enter, the fault will be injected before the syscall is executed. if the flag is exit, the fault will be injected after the syscall is executed",
					Required: true,
				},
				 &spec.ExpFlag{
					Name: "first",
					Desc: "if the flag is set, the fault will be injected to the first met syscall",
				 },
				 &spec.ExpFlag{
					Name: "end",
					Desc: "if the flag is set, the fault will be injected to the last met syscall",
				 },
				 &spec.ExpFlag{
					Name: "step",
					Desc: "the fault will be injected intervally",
				 },
			 },
			 ActionExecutor: &StraceDelayActionExecutor{},
			 ActionExample: `
# Create a strace 10s delay experiment to the process
blade create strace delay --pid 1 --syscall-name mmap --time 10s`,
			 ActionPrograms: []string{StraceDelayBin},
		 },
	 }
 }

 func (*StraceDelayActionSpec) Name() string {
	return "delay"
}

func (*StraceDelayActionSpec) Aliases() []string {
	return []string{}
}

func (*StraceDelayActionSpec) ShortDesc() string {
	return "Delay the syscall of the target pid"
}

func (f *StraceDelayActionSpec) LongDesc() string {
	if f.ActionLongDesc != "" {
		return f.ActionLongDesc
	}
	return "Delay syscall of the specified process, if the process exists"
}

type StraceDelayActionExecutor struct {
	channel spec.Channel
}


func (dae *StraceDelayActionExecutor) SetChannel(channel spec.Channel) {
	dae.channel = channel
}

func (*StraceDelayActionExecutor) Name() string {
	return "delay"
}

func (dae *StraceDelayActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response{
	if dae.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError],"channel is nil")
	}

	var pidList string
	var delay_loc_flag string
	var first_flag string
	var end_flag string
	var step string
	pidStr := model.ActionFlags["pid"]
	if pidStr != ""{
		pids, err := util.ParseIntegerListToStringSlice(pidStr)
		if err != nil {
			return spec.ReturnFail(spec.Code[spec.IllegalParameters],
				fmt.Sprintf("parse %s flag err, %v", "pid",err))
		}
		pidList = strings.Join(pids, ",")
	}
	time := model.ActionFlags["time"]
	if time == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --time flag")
	}
	syscallName := model.ActionFlags["syscall-name"]
	if syscallName == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --syscall-name flag")
	}

	delay_loc_flag = model.ActionFlags["delay-loc"]
	if delay_loc_flag == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --delay-loc flag")
	}
	first_flag = model.ActionFlags["first"]
	end_flag = model.ActionFlags["end"]
	step = model.ActionFlags["step"]
	if _, ok := spec.IsDestroy(ctx); ok{
		return dae.stop(ctx, pidList, syscallName)
	}
	return dae.start(ctx, pidList, time, syscallName,delay_loc_flag,first_flag,end_flag,step)
}

//start strace delay
func (dae *StraceDelayActionExecutor) start(ctx context.Context, pidList string,time string, syscallName string, delay_loc_flag string, first_flag string, end_flag string, step string) *spec.Response{
	args := fmt.Sprintf("--start --pid %s --time %s --syscall-name %s --delay-loc %s",pidList, time, syscallName, delay_loc_flag)
	if first_flag != ""{
		args = fmt.Sprintf("%s --first %s", args, first_flag)
	}
	if end_flag != ""{
		args = fmt.Sprintf("%s --end %s",args,end_flag)
	}
	if step != ""{
		args = fmt.Sprintf("%s --step %s",args,step)
	}
	// fmt.Println(args)
	return dae.channel.Run(ctx,path.Join(dae.channel.GetScriptPath(), StraceDelayBin), args)
}

func (dae *StraceDelayActionExecutor) stop(ctx context.Context, pidList string,  syscallName string) *spec.Response{
	args := fmt.Sprintf("--stop --pid %s --syscall-name %s",pidList,syscallName)
	return dae.channel.Run(ctx, path.Join(dae.channel.GetScriptPath(), StraceDelayBin),args)
}
