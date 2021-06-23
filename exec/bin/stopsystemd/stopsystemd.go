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
	 "flag"
	 "fmt"
	 "context"

	 "github.com/chaosblade-io/chaosblade-spec-go/channel"
	 "github.com/chaosblade-io/chaosblade-exec-os/exec/bin"
 )
 
 var service string
 
 func main() {
	 flag.StringVar(&service, "service", "", "service name")
	 bin.ParseFlagAndInitLog()
 
	 stopSystemd(service)
 }
 
 var cl = channel.NewLocalChannel()
 
 func stopSystemd(service string) {
	var ctx = context.Background()
	 response := cl.Run(ctx, "systemctl", fmt.Sprintf("stop %s", service))
	 if !response.Success {
		 bin.PrintErrAndExit(response.Err)
		 return
	 }
	 bin.PrintOutputAndExit(response.Result.(string))
 }
 