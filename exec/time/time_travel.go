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
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
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
					Desc: "Travel time offset, for example: -2h3m50s",
				},
				&spec.ExpFlag{
					Name: "disableNtp",
					Desc: "Whether to disable Network Time Protocol to synchronize time (default: true, set to false if NTP is not supported)",
				},
			},
			ActionExecutor: &TravelTimeExecutor{},
			ActionExample: `
# Time travel 5 minutes and 30 seconds into the future
blade create time travel --offset 5m30s

# Time travel without disabling NTP (useful when NTP is not supported)
blade create time travel --offset 5m30s --disableNtp false

# Time travel backward 2 hours and 30 minutes
blade create time travel --offset -2h30m
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
	return "Modify system time to fake processes. Supports multiple time formats and gracefully handles systems without timedatectl or NTP support."
}

func (*TravelTimeActionCommandSpec) Categories() []string {
	return []string{category.SystemTime}
}

type TravelTimeExecutor struct {
	channel spec.Channel
}

func (tte *TravelTimeExecutor) Name() string {
	return "travel"
}

func (tte *TravelTimeExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	// Check for available time modification commands
	commands := []string{"date"}
	if response, ok := tte.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	// Check if timedatectl is available (optional)
	timedatectlAvailable := tte.channel.IsCommandAvailable(ctx, "timedatectl")
	log.Infof(ctx, "timedatectl available: %v", timedatectlAvailable)

	var disableNtp bool
	timeOffsetStr := model.ActionFlags["offset"]
	disableNtpStr := model.ActionFlags["disableNtp"]

	if timeOffsetStr == "" {
		log.Errorf(ctx, "offset is nil")
		return spec.ResponseFailWithFlags(spec.ParameterLess, "offset")
	}
	disableNtp = disableNtpStr == "true" || disableNtpStr == ""

	if _, ok := spec.IsDestroy(ctx); ok {
		return tte.stop(ctx, timedatectlAvailable)
	}

	return tte.start(ctx, timeOffsetStr, disableNtp, timedatectlAvailable)
}

func (tte *TravelTimeExecutor) SetChannel(channel spec.Channel) {
	tte.channel = channel
}

func (tte *TravelTimeExecutor) stop(ctx context.Context, timedatectlAvailable bool) *spec.Response {
	// Try to re-enable NTP if timedatectl is available
	if timedatectlAvailable {
		response := tte.channel.Run(ctx, "timedatectl", `set-ntp true`)
		if !response.Success {
			// Check if the error is due to NTP not being supported
			if strings.Contains(response.Err, "NTP not supported") {
				log.Warnf(ctx, "NTP is not supported on this system, skipping NTP re-enable")
			} else {
				// For other errors, still return the error
				return response
			}
		}
	}

	// Sync hardware clock with system time
	return tte.channel.Run(ctx, "hwclock", `--hctosys`)
}

func (tte *TravelTimeExecutor) start(ctx context.Context, timeOffsetStr string, disableNtp bool, timedatectlAvailable bool) *spec.Response {
	duration, err := time.ParseDuration(timeOffsetStr)
	if err != nil {
		log.Errorf(ctx, "offset is invalid")
		return spec.ResponseFailWithFlags(spec.ParameterInvalid, "offset", timeOffsetStr, err)
	}

	// Calculate target time
	targetTime := time.Now().Add(duration)

	// Try to disable NTP if requested and timedatectl is available
	if disableNtp && timedatectlAvailable {
		response := tte.channel.Run(ctx, "timedatectl", `set-ntp false`)
		if !response.Success {
			// Check if the error is due to NTP not being supported
			if strings.Contains(response.Err, "NTP not supported") {
				log.Warnf(ctx, "NTP is not supported on this system, continuing without disabling NTP")
			} else {
				// For other errors, still return the error
				return response
			}
		}
	}

	// Set system time using multiple format attempts for better compatibility
	return tte.setSystemTime(ctx, targetTime)
}

// setSystemTime attempts to set system time using multiple methods for better compatibility
func (tte *TravelTimeExecutor) setSystemTime(ctx context.Context, targetTime time.Time) *spec.Response {
	// Try different time formats for better compatibility
	timeFormats := []string{
		"2006-01-02 15:04:05", // ISO format
		"01/02/2006 15:04:05", // Original format
		"2006-01-02T15:04:05", // ISO without space
		"Jan 2 15:04:05 2006", // Unix date format
	}

	var lastError error
	for _, format := range timeFormats {
		timeStr := targetTime.Format(format)
		log.Infof(ctx, "Attempting to set time with format '%s': %s", format, timeStr)

		response := tte.channel.Run(ctx, "date", fmt.Sprintf(`-s "%s"`, timeStr))
		if response.Success {
			log.Infof(ctx, "Successfully set time using format: %s", format)
			return response
		}
		lastError = fmt.Errorf("format %s failed: %s", format, response.Err)
		log.Warnf(ctx, "Time format '%s' failed: %s", format, response.Err)
	}

	// If all formats failed, try using timedatectl if available
	if tte.channel.IsCommandAvailable(ctx, "timedatectl") {
		isoTime := targetTime.Format("2006-01-02 15:04:05")
		log.Infof(ctx, "Attempting to set time using timedatectl: %s", isoTime)
		response := tte.channel.Run(ctx, "timedatectl", fmt.Sprintf(`set-time "%s"`, isoTime))
		if response.Success {
			log.Infof(ctx, "Successfully set time using timedatectl")
			return response
		}
		lastError = fmt.Errorf("timedatectl failed: %s", response.Err)
		log.Warnf(ctx, "timedatectl failed: %s", response.Err)
	}

	return spec.ResponseFailWithFlags(spec.OsCmdExecFailed,
		fmt.Sprintf("Failed to set system time with all available methods. Last error: %v", lastError))
}
