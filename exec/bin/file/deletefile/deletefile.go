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
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"path"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
)

var filepath string
var appendFileStart, appendFileStop, force bool

func main() {
	flag.StringVar(&filepath, "filepath", "", "filepath")
	flag.BoolVar(&force, "force", false, "force remove can't be restored")
	flag.BoolVar(&appendFileStart, "start", false, "start append file")
	flag.BoolVar(&appendFileStop, "stop", false, "stop append file")
	bin.ParseFlagAndInitLog()

	if appendFileStart {
		if filepath == "" {
			bin.PrintErrAndExit("less --filepath flag")
		}
		startDeleteFile(filepath, force)
	} else if appendFileStop {
		stopDeleteFile(filepath, force)
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

var cl = channel.NewLocalChannel()

func startDeleteFile(filepath string, force bool) {
	ctx := context.Background()
	var response *spec.Response
	if force {
		response = cl.Run(ctx, "rm", fmt.Sprintf(`-rf "%s"`, filepath))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
	} else {
		target := path.Join(path.Dir(filepath), "."+md5Hex(path.Base(filepath)))
		response = cl.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, filepath, target))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
	}
	bin.PrintOutputAndExit(response.Result.(string))
}

func stopDeleteFile(filepath string, force bool) {
	if force {
		// nothing to do
	} else {
		ctx := context.Background()
		target := path.Join(path.Dir(filepath), "."+md5Hex(path.Base(filepath)))
		response := cl.Run(ctx, "mv", fmt.Sprintf(`"%s" "%s"`, target, filepath))
		if !response.Success {
			bin.PrintErrAndExit(response.Err)
			return
		}
		bin.PrintOutputAndExit(response.Result.(string))
	}
}

func md5Hex(s string) string {
	m := md5.New()
	m.Write([]byte (s))
	return hex.EncodeToString(m.Sum(nil))
}
