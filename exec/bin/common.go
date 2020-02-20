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

package bin

import (
	"flag"
	"fmt"
	"os"

	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

const ErrPrefix = "Error:"

var ExitFunc = os.Exit
var ExitMessageForTesting string

func PrintAndExitWithErrPrefix(message string) {
	ExitMessageForTesting = fmt.Sprintf("%s %s", ErrPrefix, message)
	fmt.Fprint(os.Stderr, fmt.Sprintf("%s %s", ErrPrefix, message))
	ExitFunc(1)
}

func PrintErrAndExit(message string) {
	ExitMessageForTesting = message
	fmt.Fprint(os.Stderr, message)
	ExitFunc(1)
}

func PrintOutputAndExit(message string) {
	ExitMessageForTesting = message
	fmt.Fprintf(os.Stdout, message)
	ExitFunc(0)
}

func ParseFlagAndInitLog() {
	util.AddDebugFlag()
	flag.Parse()
	util.InitLog(util.Bin)
}
