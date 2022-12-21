package shutdown

import (
	"context"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type PowerOffActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewPowerOffActionCommandSpec() spec.ExpActionCommandSpec {
	return &PowerOffActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &PowerOffExecutor{},
			ActionExample: `
# Force to shutdown machine
blade c shutdown poweroff --force

# Shutdown  machine after 1 minute
blade c shutdown poweroff --time 1`,
			ActionPrograms:    []string{},
			ActionCategories:  []string{category.SystemShutdown},
			ActionProcessHang: true,
		},
	}
}

func (p *PowerOffActionCommandSpec) Name() string {
	return "poweroff"
}

func (p *PowerOffActionCommandSpec) Aliases() []string {
	return []string{"p"}
}

func (p *PowerOffActionCommandSpec) ShortDesc() string {
	return "Shutdown machine"
}

func (p *PowerOffActionCommandSpec) LongDesc() string {
	return "Shutdown machine. Warning! the experiment cannot be recovered by this tool."
}

type PowerOffExecutor struct {
	channel spec.Channel
}

func (p *PowerOffExecutor) Name() string {
	return "poweroff"
}

func (p *PowerOffExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		return cancel(ctx, uid, model, p.channel)
	}
	return execute(ctx, model, "-P", p.channel)
}

func (p *PowerOffExecutor) SetChannel(channel spec.Channel) {
	p.channel = channel
}
