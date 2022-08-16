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

package time

import (
	"context"
	"fmt"
	"time"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const TravelTimeBin = "chaos_timetravel"

type TravelTimeActionCommandSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewTravelTimeActionCommandSpec() spec.ExpActionCommandSpec {
	return &TravelTimeActionCommandSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: []spec.ExpFlagSpec{},
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name: "offset",
					Desc: "Travel time offset, for example: -1d2h3m50s",
				},
				&spec.ExpFlag{
					Name: "disableNtp",
					Desc: "Whether to disable Network Time Protocol to synchronize time",
				},
			},
			ActionExecutor: &TravelTimeExecutor{},
			ActionExample: `
# Time travel 5 minutes and 30 seconds into the future
blade create time travel --offset 5m30s
`,
			ActionPrograms:   []string{TravelTimeBin},
			ActionCategories: []string{category.SystemTime},
		},
	}
}

func (*TravelTimeActionCommandSpec) Name() string {
	return "travel"
}

func (*TravelTimeActionCommandSpec) Aliases() []string {
	return []string{"k"}
}

func (*TravelTimeActionCommandSpec) ShortDesc() string {
	return "Time Travel"
}

func (k *TravelTimeActionCommandSpec) LongDesc() string {
	if k.ActionLongDesc != "" {
		return k.ActionLongDesc
	}
	return "Modify system time to fake processes"
}

func (*TravelTimeActionCommandSpec) Categories() []string {
	return []string{category.SystemProcess}
}

type TravelTimeExecutor struct {
	channel spec.Channel
}

func (tte *TravelTimeExecutor) Name() string {
	return "travel"
}

func (tte *TravelTimeExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"date", "timedatectl"}
	if response, ok := tte.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	var disableNtp bool
	timeOffsetStr := model.ActionFlags["offset"]
	disableNtpStr := model.ActionFlags["disableNtp"]

	if timeOffsetStr == "" {
		log.Errorf(ctx, "offset is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "offset")
	}
	disableNtp = disableNtpStr == "true" || disableNtpStr == ""

	if _, ok := spec.IsDestroy(ctx); ok {
		return tte.stop(ctx)
	}

	return tte.start(ctx, timeOffsetStr, disableNtp)
}

func (tte *TravelTimeExecutor) SetChannel(channel spec.Channel) {
	tte.channel = channel
}

func (tte *TravelTimeExecutor) stop(ctx context.Context) *spec.Response {
	response := tte.channel.Run(ctx, "timedatectl", fmt.Sprintf(`set-ntp true`))
	if !response.Success {
		return response
	}
	return tte.channel.Run(ctx, "hwclock", fmt.Sprintf(`--hctosys`))
}

func (tte *TravelTimeExecutor) start(ctx context.Context, timeOffsetStr string, disableNtp bool) *spec.Response {
	duration, err := time.ParseDuration(timeOffsetStr)
	if err != nil {
		log.Errorf(ctx, "offset is invalid")
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "offset", timeOffsetStr, err)
	}
	targetTime := time.Now().Add(duration).Format("01/02/2006 15:04:05")

	if disableNtp {
		response := tte.channel.Run(ctx, "timedatectl", fmt.Sprintf(`set-ntp false`))
		if !response.Success {
			return response
		}
	}

	return tte.channel.Run(ctx, "date", fmt.Sprintf(`-s "%s" `, targetTime))
}
