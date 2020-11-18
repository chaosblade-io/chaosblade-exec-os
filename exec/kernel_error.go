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

 const StraceErrorBin = "chaos_straceerror"

 type StraceErrorActionSpec struct {
	 spec.BaseExpActionCommandSpec
 }

 func NewStraceErrorActionSpec() spec.ExpActionCommandSpec {
	 return &StraceErrorActionSpec{
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
					Name: "return-value",
					Desc: "the return-value the syscall will return",
					Required: true,
				 },
				  &spec.ExpFlag{
					Name: "first",
					Desc: "if the flag is true, the fault will be injected to the first met syscall",
				 },
				 &spec.ExpFlag{
					Name: "end",
					Desc: "if the flag is true, the fault will be injected to the last met syscall",
				 },
				 &spec.ExpFlag{
					Name: "step",
					Desc: "the fault will be injected intervally",
				 },
			 },
			 ActionExecutor: &StraceErrorActionExecutor{},
			 ActionExample: `
# Create a strace error experiment to the process
blade create strace error --pid 1 --syscall-name mmap --return-value XX`,
			 ActionPrograms: []string{StraceErrorBin},
		 },
	 }
 }

 func (*StraceErrorActionSpec) Name() string {
	return "error"
}

func (*StraceErrorActionSpec) Aliases() []string {
	return []string{}
}

func (*StraceErrorActionSpec) ShortDesc() string {
	return "change the syscall's return value of the target pid"
}

func (f *StraceErrorActionSpec) LongDesc() string {
	if f.ActionLongDesc != "" {
		return f.ActionLongDesc
	}
	return "change the syscall's return value of the specified process, if the process exists"
}

type StraceErrorActionExecutor struct {
	channel spec.Channel
}


func (dae *StraceErrorActionExecutor) SetChannel(channel spec.Channel) {
	dae.channel = channel
}

func (*StraceErrorActionExecutor) Name() string {
	return "error"
}

func (dae *StraceErrorActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response{
	if dae.channel == nil {
		return spec.ReturnFail(spec.Code[spec.ServerError],"channel is nil")
	}

	var pidList string
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
	return_value := model.ActionFlags["return-value"]
	if return_value == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --return-value flag")
	}
	syscallName := model.ActionFlags["syscall-name"]
	if syscallName == "" {
		return spec.ReturnFail(spec.Code[spec.IllegalParameters], "must specify --syscall-name flag")
	}

	first_flag = model.ActionFlags["first"]
	end_flag = model.ActionFlags["end"]

	step = model.ActionFlags["step"]

	if _, ok := spec.IsDestroy(ctx); ok{
		return dae.stop(ctx, pidList, syscallName)
	}
	// fmt.Printf("%s,%s,%s",first_flag,end_flag,step)
	return dae.start(ctx, pidList, return_value, syscallName,first_flag,end_flag,step)
}

//start strace Error
func (dae *StraceErrorActionExecutor) start(ctx context.Context, pidList string,return_value string, syscallName string,first_flag string, end_flag string, step string) *spec.Response{
	args := fmt.Sprintf("--start --pid %s --return-value %s --syscall-name %s ",pidList, return_value, syscallName)
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
	return dae.channel.Run(ctx,path.Join(dae.channel.GetScriptPath(), StraceErrorBin), args)
}

func (dae *StraceErrorActionExecutor) stop(ctx context.Context, pidList string,  syscallName string) *spec.Response{
	args := fmt.Sprintf("--stop --pid %s --syscall-name %s",pidList,syscallName)
	return dae.channel.Run(ctx, path.Join(dae.channel.GetScriptPath(), StraceErrorBin),args)
}
