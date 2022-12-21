package shutdown

import (
	"context"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type RebootActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewRebootActionCommandSpec() spec.ExpActionCommandSpec {
	return &RebootActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &RebootExecutor{},
			ActionExample: `
# Force to reboot machine
blade c shutdown reboot --force

# Reboot machine after 1 minute
blade c shutdown reboot --time 1`,
			ActionPrograms:    []string{},
			ActionCategories:  []string{category.SystemShutdown},
			ActionProcessHang: true,
		},
	}
}

func (r *RebootActionCommandSpec) Name() string {
	return "reboot"
}

func (r *RebootActionCommandSpec) Aliases() []string {
	return []string{"s"}
}

func (r *RebootActionCommandSpec) ShortDesc() string {
	return "Reboot machine"
}

func (r *RebootActionCommandSpec) LongDesc() string {
	return "Reboot machine. Warning! the experiment cannot be recovered by this tool."
}

type RebootExecutor struct {
	channel spec.Channel
}

func (r *RebootExecutor) Name() string {
	return "reboot"
}

func (r *RebootExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		return cancel(ctx, uid, model, r.channel)
	}
	return execute(ctx, model, "-r", r.channel)
}

func (r *RebootExecutor) SetChannel(channel spec.Channel) {
	r.channel = channel
}
