package exec

import (
	"context"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"strings"
)

// todo
var cl = channel.NewLocalChannel()

// stop hang process
func Destroy(ctx context.Context, c spec.Channel, action string) *spec.Response {
	ctx = context.WithValue(ctx, channel.ProcessKey, action)
	pids, _ := cl.GetPidsByProcessName("chaos_os", ctx)
	if pids == nil || len(pids) == 0 {
		sprintf := fmt.Sprintf("destory experiment failed, cannot get the chaos_os program")
		return spec.ReturnFail(spec.OsCmdExecFailed, sprintf)
	}
	return cl.Run(ctx, "kill", fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
}
