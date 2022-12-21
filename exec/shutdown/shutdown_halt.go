package shutdown

import (
	"context"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

type HaltActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewHaltActionCommandSpec() spec.ExpActionCommandSpec {
	return &HaltActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags:    []spec.ExpFlagSpec{},
			ActionExecutor: &HaltExecutor{},
			ActionExample: `
# Force to halt machine
blade c shutdown halt --force

# halt machine after 1 minute
blade c shutdown halt --time 1`,
			ActionPrograms:    []string{},
			ActionCategories:  []string{category.SystemShutdown},
			ActionProcessHang: true,
		},
	}
}

func (h *HaltActionCommandSpec) Name() string {
	return "halt"
}

func (h *HaltActionCommandSpec) Aliases() []string {
	return []string{"h"}
}

func (h *HaltActionCommandSpec) ShortDesc() string {
	return "Halt machine"
}

func (h *HaltActionCommandSpec) LongDesc() string {
	return "Halt machine. Warning! the experiment cannot be recovered by this tool."
}

type HaltExecutor struct {
	channel spec.Channel
}

func (h *HaltExecutor) Name() string {
	return "halt"
}

func (h *HaltExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	if _, ok := spec.IsDestroy(ctx); ok {
		return cancel(ctx, uid, model, h.channel)
	}
	return execute(ctx, model, "-H", h.channel)
}

func (h *HaltExecutor) SetChannel(channel spec.Channel) {
	h.channel = channel
}
