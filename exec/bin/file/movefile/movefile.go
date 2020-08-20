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

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"path"
)

var target, filepath string
var appendFileStart, appendFileStop, force bool

func main() {
	flag.StringVar(&target, "target", "", "content")
	flag.StringVar(&filepath, "filepath", "", "filepath")
	flag.BoolVar(&force, "force", false, "overwrite target file")
	flag.BoolVar(&appendFileStart, "start", false, "start append file")
	flag.BoolVar(&appendFileStop, "stop", false, "stop append file")
	bin.ParseFlagAndInitLog()

	if appendFileStart {
		if target == "" || filepath == "" {
			bin.PrintErrAndExit("less --target or --filepath flag")
		}
		startMoveFile(filepath, target, force)
	} else if appendFileStop {
		stopMoveFile(filepath, target)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func startMoveFile(filepath, target string, force bool) {
	ctx := context.Background()
	var response *spec.Response
	if force {
		response = cl.Run(ctx, "mv", fmt.Sprintf(`-f "%s" "%s"`, filepath, target))
	} else {
		response = cl.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, filepath, target))
	}
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func stopMoveFile(filepath, target string) {
	origin := path.Join(target, "/", path.Base(filepath))

	ctx := context.Background()
	response := cl.Run(ctx, "mv", fmt.Sprintf(`-f "%s" "%s"`, origin, path.Dir(filepath)))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}

	bin.PrintOutputAndExit(response.Result.(string))
}
