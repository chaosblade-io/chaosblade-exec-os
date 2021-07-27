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
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var mark, filepath string
var appendFileStart, appendFileStop bool

func main() {
	flag.StringVar(&mark, "mark", "", "content")
	flag.StringVar(&filepath, "filepath", "", "filepath")
	flag.BoolVar(&appendFileStart, "start", false, "start append file")
	flag.BoolVar(&appendFileStop, "stop", false, "stop append file")
	bin.ParseFlagAndInitLog()

	if appendFileStart {
		if mark == "" || filepath == "" {
			bin.PrintErrAndExit("less --mark or --filepath flag")
		}
		startChmodFile(filepath, mark)
	} else if appendFileStop {
		stopChmodFile(filepath)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

const tmpFileChmod = "/tmp/chaos-file-chmod.tmp"

func startChmodFile(filepath, mark string) {
	ctx := context.Background()

	response := cl.Run(ctx, "grep", fmt.Sprintf(`-q "%s:" "%s"`, filepath, tmpFileChmod))
	if response.Success {
		bin.PrintErrAndExit(fmt.Sprintf("%s is already being experimented o", filepath))
		return
	}

	fileInfo, _ := os.Stat(filepath)
	originMark := strconv.FormatInt(int64(fileInfo.Mode().Perm()), 8)

	response = cl.Run(ctx, "echo", fmt.Sprintf(`%s:%s >> %s`, filepath, originMark, tmpFileChmod))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	response = cl.Run(ctx, "chmod", fmt.Sprintf(`%s "%s"`, mark, filepath))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func stopChmodFile(filepath string) {

	ctx := context.Background()
	// get origin mark
	response := cl.Run(ctx, "grep", fmt.Sprintf(`%s: %s | awk -F ':' '{printf $2}'`, filepath, tmpFileChmod))
	if !response.Success {
		clearTempFile(filepath, response, ctx)
		bin.PrintErrAndExit(response.Err)
		return
	}

	originMark := response.Result.(string)
	match, _ := regexp.MatchString("^([0-7]{3})$", originMark)
	if !match {
		bin.PrintErrAndExit(fmt.Sprintf("the %s mark is fail", mark))
		return
	}

	response = cl.Run(ctx, "chmod", fmt.Sprintf(`%s %s`, originMark, filepath))
	if !response.Success {
		clearTempFile(filepath, response, ctx)
		bin.PrintErrAndExit(response.Err)
		return
	}
	response, done := clearTempFile(filepath, response, ctx)
	if done {
		return
	}

	bin.PrintOutputAndExit(originMark)
}

func clearTempFile(filepath string, response *spec.Response, ctx context.Context) (*spec.Response, bool) {
	if !cl.IsCommandAvailable("cat") {
		bin.PrintErrAndExit(spec.CommandCatNotFound.Msg)
	}

	response = cl.Run(ctx, "cat", fmt.Sprintf(`"%s"| grep -v %s:`, tmpFileChmod, filepath))
	if !response.Success {
		response = cl.Run(ctx, "rm", fmt.Sprintf(`-rf "%s"`, tmpFileChmod))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return nil, true
		}
	} else {
		response = cl.Run(ctx, "echo", fmt.Sprintf(`"%s" > %s`,
			strings.TrimRight(response.Result.(string), "\n"),
			tmpFileChmod))

		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return nil, true
		}
	}
	return response, false
}
