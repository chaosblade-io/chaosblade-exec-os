package process

import (
	"context"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/shirou/gopsutil/process"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
)

const StopProcessBin = "chaos_stopprocess"

type StopProcessActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewStopProcessActionCommandSpec() spec.ExpActionCommandSpec {
	return &StopProcessActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "process",
					Desc: "Process name",
				},
				&spec.ExpFlag{
					Name: "process-cmd",
					Desc: "Process name in command",
				},
				&spec.ExpFlag{
					Name: "count",
					Desc: "Limit count, 0 means unlimited",
				},
				&spec.ExpFlag{
					Name: "local-port",
					Desc: "Local service ports. Separate multiple ports with commas (,) or connector representing ranges, for example: 80,8000-8080",
				},
			},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &StopProcessExecutor{},
			ActionExample: `
# Pause the process that contains the "SimpleHTTPServer" keyword
blade create process stop --process SimpleHTTPServer

# Pause the Java process
blade create process stop --process-cmd java.exe

# Return success even if the process not found
blade create process stop --process demo.exe --ignore-not-found`,
			ActionPrograms:   []string{StopProcessBin},
			ActionCategories: []string{category.SystemProcess},
		},
	}
}

func (*StopProcessActionCommandSpec) Name() string {
	return "stop"
}

func (*StopProcessActionCommandSpec) Aliases() []string {
	return []string{"f"}
}

func (*StopProcessActionCommandSpec) ShortDesc() string {
	return "process fake death"
}

func (s *StopProcessActionCommandSpec) LongDesc() string {
	if s.ActionLongDesc != "" {
		return s.ActionLongDesc
	}
	return "process fake death by process id or process name"
}

type StopProcessExecutor struct {
	channel spec.Channel
}

func (spe *StopProcessExecutor) Name() string {
	return "stop"
}

func (spe *StopProcessExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	resp := getPids(ctx, spe.channel, model, uid)
	if !resp.Success {
		return resp
	}
	pids := resp.Result.(string)

	if _, ok := spec.IsDestroy(ctx); ok {
		return spe.ResumeProcessByPids(ctx, pids)
	} else {
		return spe.StopProcessByPids(ctx, pids)
	}
}

func (spe *StopProcessExecutor) StopProcessByPids(ctx context.Context, pids string) *spec.Response {
	arrPids := strings.Split(pids, " ")
	for _, pid := range arrPids {
		ipid, err := strconv.ParseInt(pid, 10, 32)
		if err != nil {
			log.Errorf(ctx, "`%s`, get pid failed, err: %s", pids, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID", err.Error())
		}

		prce, err := process.NewProcessWithContext(context.Background(), int32(ipid))
		if err != nil {
			log.Errorf(ctx, "`%s`, get pid info failed, err: %s", pids, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID info", err.Error())
		}

		// stop
		err = prce.Suspend()
		if err != nil {
			log.Errorf(ctx, "`%v`, stop process failed, err: %s", pid, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "stop pid", err.Error())
		}
	}
	return spec.Success()
}

func (spe *StopProcessExecutor) ResumeProcessByPids(ctx context.Context, pids string) *spec.Response {
	arrPids := strings.Split(pids, " ")
	for _, pid := range arrPids {
		ipid, err := strconv.ParseInt(pid, 10, 32)
		if err != nil {
			log.Errorf(ctx, "`%s`, get pid failed, err: %s", pids, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID", err.Error())
		}

		prce, err := process.NewProcessWithContext(context.Background(), int32(ipid))
		if err != nil {
			log.Errorf(ctx, "`%s`, get pid info failed, err: %s", pids, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "Get PID info", err.Error())
		}

		// Resume sends SIGCONT to the process.
		err = prce.Resume()
		if err != nil {
			log.Errorf(ctx, "`%v`, restart process failed, err: %s", pid, err.Error())
			return spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "restart process", err.Error())
		}
	}
	return spec.Success()
}
func (spe *StopProcessExecutor) SetChannel(channel spec.Channel) {
	spe.channel = channel
}
