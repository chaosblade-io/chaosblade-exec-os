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

package cpu

import (
	"context"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/shirou/gopsutil/cpu"
	"time"
)

func getUsed(ctx context.Context, percpu bool, cpuIndex int) float64 {
	totalCpuPercent, err := cpu.Percent(time.Second, percpu)
	if err != nil {
		log.Fatalf(ctx, "get cpu usage fail, %s", err.Error())
	}
	if percpu {
		if cpuIndex > len(totalCpuPercent) {
			log.Fatalf(ctx, "illegal cpu index %d", cpuIndex)
		}
		return totalCpuPercent[cpuIndex]
	}
	return totalCpuPercent[0]
}
