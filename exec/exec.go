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

func CheckFilepathExists(ctx context.Context, cl spec.Channel, filepath string) bool {
	response := cl.Run(ctx, fmt.Sprintf("[ -e %s ] && echo true || echo false", filepath), "")
	if response.Success && strings.Contains(response.Result.(string), "true") {
		return true
	}
	return false
}