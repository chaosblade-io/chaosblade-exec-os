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
	"encoding/base64"
	"flag"
	"fmt"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var filepath, content string
var appendFileStart, appendFileStop, directory, enableBase64, autoCreateDir bool

func main() {
	flag.StringVar(&filepath, "filepath", "", "filepath")
	flag.StringVar(&content, "content", "", "content")
	flag.BoolVar(&directory, "directory", false, "is dir")
	flag.BoolVar(&enableBase64, "enable-base64", false, "content support base64 encoding")
	flag.BoolVar(&autoCreateDir, "auto-create-dir", false, "automatically creates a directory that does not exist")
	flag.BoolVar(&appendFileStart, "start", false, "start append file")
	flag.BoolVar(&appendFileStop, "stop", false, "stop append file")
	bin.ParseFlagAndInitLog()

	if appendFileStart {
		if filepath == "" {
			bin.PrintErrAndExit("less --filepath flag")
		}
		startAddFile(filepath, content, directory, enableBase64, autoCreateDir)
	} else if appendFileStop {
		stopAddFile(filepath)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func startAddFile(filepath, content string, directory, enableBase64, autoCreateDir bool) {
	ctx := context.Background()

	var response *spec.Response
	dir := path.Dir(filepath)
	if autoCreateDir && !util.IsExist(dir) {
		response = cl.Run(ctx, "mkdir", fmt.Sprintf(`-p %s`, dir))
	}
	if directory {
		response = cl.Run(ctx, "mkdir", fmt.Sprintf(`%s`, filepath))
	} else {
		if content == "" {
			response = cl.Run(ctx, "touch", fmt.Sprintf(`%s`, filepath))
		} else {
			if enableBase64 {
				decodeBytes, err := base64.StdEncoding.DecodeString(content)
				if err != nil {
					bin.PrintErrAndExit(err.Error())
					return
				}
				content = string(decodeBytes)
			}
			response = cl.Run(ctx, "echo", fmt.Sprintf(`"%s" >> "%s"`, content, filepath))
		}
	}
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func stopAddFile(filepath string) {

	ctx := context.Background()
	// get origin mark
	response := cl.Run(ctx, "rm", fmt.Sprintf(`-rf %s`, filepath))
	if !response.Success {
		bin.PrintErrAndExit(response.Err)
		return
	}
	bin.PrintOutputAndExit(response.Result.(string))
}
