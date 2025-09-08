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

package file

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chaosblade-io/chaosblade-exec-os/exec"
	"github.com/chaosblade-io/chaosblade-spec-go/log"

	"github.com/chaosblade-io/chaosblade-exec-os/exec/category"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

const AppendFileBin = "chaos_appendfile"

type FileAppendActionSpec struct {
	spec.BaseExpActionCommandSpec
}

func NewFileAppendActionSpec() spec.ExpActionCommandSpec {
	return &FileAppendActionSpec{
		spec.BaseExpActionCommandSpec{
			ActionMatchers: fileCommFlags,
			ActionFlags: []spec.ExpFlagSpec{
				&spec.ExpFlag{
					Name:     "content",
					Desc:     "append content",
					Required: true,
				},
				&spec.ExpFlag{
					Name: "count",
					Desc: "the number of append count, must be a positive integer, default 1",
				},
				&spec.ExpFlag{
					Name: "interval",
					Desc: "append interval, must be a positive integer",
				},
				&spec.ExpFlag{
					Name:   "escape",
					Desc:   "symbols to escape, use --escape, at this --count is invalid",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "enable-base64",
					Desc:   "append content enable base64 encoding",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:     "cgroup-root",
					Desc:     "cgroup root path, default value /sys/fs/cgroup",
					NoArgs:   false,
					Required: false,
					Default:  "/sys/fs/cgroup",
				},
				&spec.ExpFlag{
					Name:   "enable-backup",
					Desc:   "enable backup original file for restore on destroy, default false",
					NoArgs: true,
				},
				&spec.ExpFlag{
					Name:   "delete-file",
					Desc:   "delete file on destroy operation, default false. When used with enable-backup, this parameter has higher priority",
					NoArgs: true,
				},
			},
			ActionExecutor: &FileAppendActionExecutor{},
			ActionExample: `
# Appends the content "HELLO WORLD" to the /home/logs/nginx.log file (creates file if not exists)
blade create file append --filepath=/home/logs/nginx.log --content="HELLO WORLD"

# Appends the content "HELLO WORLD" to the /home/logs/nginx.log file, interval 10 seconds
blade create file append --filepath=/home/logs/nginx.log --content="HELLO WORLD" --interval 10

# Appends the content "HELLO WORLD" to the /home/logs/nginx.log file, enable base64 encoding
blade create file append --filepath=/home/logs/nginx.log --content=SEVMTE8gV09STEQ=

# Appends content with backup (destroy will restore original file or delete created file)
blade create file append --filepath=/home/logs/nginx.log --content="HELLO WORLD" --enable-backup=true

# Appends content and delete file on destroy (delete-file has higher priority than enable-backup)
blade create file append --filepath=/home/logs/nginx.log --content="HELLO WORLD" --delete-file=true

# Appends content with backup but preserve file on destroy (delete-file=false overrides enable-backup=true)
blade create file append --filepath=/home/logs/nginx.log --content="HELLO WORLD" --enable-backup=true --delete-file=false

# mock interface timeout exception
blade create file append --filepath=/home/logs/nginx.log --content="@{DATE:+%Y-%m-%d %H:%M:%S} ERROR invoke getUser timeout [@{RANDOM:100-200}]ms abc  mock exception"
`,
			ActionPrograms:    []string{AppendFileBin},
			ActionCategories:  []string{category.SystemFile},
			ActionProcessHang: true,
		},
	}
}

func (*FileAppendActionSpec) Name() string {
	return "append"
}

func (*FileAppendActionSpec) Aliases() []string {
	return []string{}
}

func (*FileAppendActionSpec) ShortDesc() string {
	return "File content append"
}

func (f *FileAppendActionSpec) LongDesc() string {
	return "File content append. "
}

type FileAppendActionExecutor struct {
	channel spec.Channel
}

func (*FileAppendActionExecutor) Name() string {
	return "append"
}

func (f *FileAppendActionExecutor) Exec(uid string, ctx context.Context, model *spec.ExpModel) *spec.Response {
	commands := []string{"echo", "kill", "mkdir"}
	if response, ok := f.channel.IsAllCommandsAvailable(ctx, commands); !ok {
		return response
	}

	filepath := model.ActionFlags["filepath"]
	if _, ok := spec.IsDestroy(ctx); ok {
		enableBackup := model.ActionFlags["enable-backup"] == "true" // default false
		deleteFile := model.ActionFlags["delete-file"] == "true"     // default false
		return f.stop(filepath, enableBackup, deleteFile, ctx)
	}

	// File append operation supports creating new files if they don't exist
	// The echo command with >> redirection will automatically create the file

	// default 1
	count := 1
	// default 0
	interval := 0

	content := model.ActionFlags["content"]
	countStr := model.ActionFlags["count"]
	intervalStr := model.ActionFlags["interval"]
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count < 1 {
			log.Errorf(ctx, "`%s` value must be a positive integer", "count")
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "count", count, "it must be a positive integer")
		}
	}
	if intervalStr != "" {
		var err error
		interval, err = strconv.Atoi(intervalStr)
		if err != nil || interval < 1 {
			log.Errorf(ctx, "`%s` value must be a positive integer", "interval")
			return spec.ResponseFailWithFlags(spec.ParameterIllegal, "interval", interval, "it must be a positive integer")
		}
	}

	escape := model.ActionFlags["escape"] == "true"
	enableBase64 := model.ActionFlags["enable-base64"] == "true"
	enableBackup := model.ActionFlags["enable-backup"] == "true" // default false

	return f.start(filepath, content, count, interval, escape, enableBase64, enableBackup, ctx)
}

func (f *FileAppendActionExecutor) start(filepath string, content string, count int, interval int, escape bool, enableBase64 bool, enableBackup bool, ctx context.Context) *spec.Response {
	// Create backup of original file before appending content (if enabled and file exists)
	if enableBackup {
		uid := ctx.Value(spec.Uid)
		if uid != nil && uid != spec.UnknownUid && uid != "" {
			// Only create backup if the original file exists
			if exec.CheckFilepathExists(ctx, f.channel, filepath) {
				backupFile := filepath + ".chaos-blade-backup-" + uid.(string)
				// Only create backup if it doesn't exist (to avoid overwriting existing backup)
				if !exec.CheckFilepathExists(ctx, f.channel, backupFile) {
					response := f.channel.Run(ctx, "cp", fmt.Sprintf(`"%s" "%s"`, filepath, backupFile))
					if !response.Success {
						log.Errorf(ctx, "Failed to create backup file: %s", response.Err)
						// Continue with append operation even if backup fails
					} else {
						log.Infof(ctx, "Created backup file: %s", backupFile)
					}
				}
			} else {
				log.Infof(ctx, "File does not exist, skipping backup creation: %s", filepath)
			}
		}
	}

	// first append
	response := appendFile(f.channel, count, ctx, content, filepath, escape, enableBase64)
	if !response.Success {
		return response
	}
	// Without interval, it will not be executed regularly.
	if interval < 1 {
		return nil
	}

	// For interval-based operations, we need to run in a loop
	// This will be managed by the chaos_os process
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			response := appendFile(f.channel, count, ctx, content, filepath, escape, enableBase64)
			if !response.Success {
				log.Errorf(ctx, "Failed to append file content: %s", response.Err)
				// Continue running even if one append fails
			}
		case <-ctx.Done():
			// Context cancelled, stop the ticker
			log.Infof(ctx, "File append interval operation stopped")
			return nil
		}
	}
}

func (f *FileAppendActionExecutor) stop(filepath string, enableBackup bool, deleteFile bool, ctx context.Context) *spec.Response {
	// For file append operation, we need to handle both one-time and interval-based operations
	// If it's an interval-based operation, we need to stop the chaos_os process first

	// Check if this is an interval-based operation by looking for the process
	ctx = context.WithValue(ctx, "bin", AppendFileBin)
	response := exec.Destroy(ctx, f.channel, "file append")

	// If the destroy operation failed (no process found), it might be a one-time operation
	// In that case, we handle file restoration/deletion based on backup settings
	if !response.Success {
		log.Infof(ctx, "No running process found, treating as one-time operation")
		return f.handleOneTimeOperation(filepath, enableBackup, deleteFile, ctx)
	}

	// For interval-based operations, we also need to handle file restoration/deletion
	return f.handleOneTimeOperation(filepath, enableBackup, deleteFile, ctx)
}

func (f *FileAppendActionExecutor) handleOneTimeOperation(filepath string, enableBackup bool, deleteFile bool, ctx context.Context) *spec.Response {
	// Priority logic: delete-file parameter has higher priority than enable-backup
	if deleteFile {
		// If delete-file is true, handle based on backup settings
		if enableBackup {
			// Get the experiment UID to find the backup file
			uid := ctx.Value(spec.Uid)
			if uid == nil || uid == spec.UnknownUid || uid == "" {
				log.Errorf(ctx, "Cannot get experiment UID for file append destroy")
				return spec.ReturnFail(spec.ParameterInvalid, "experiment UID is required for destroy operation")
			}

			// The backup file should be stored as .chaos-blade-backup-{uid}
			backupFile := filepath + ".chaos-blade-backup-" + uid.(string)

			// Check if backup file exists
			if !exec.CheckFilepathExists(ctx, f.channel, backupFile) {
				// If no backup file exists, it means the original file didn't exist
				// In this case, we should delete the file that was created by the append operation
				if exec.CheckFilepathExists(ctx, f.channel, filepath) {
					response := f.channel.Run(ctx, "rm", fmt.Sprintf(`"%s"`, filepath))
					if !response.Success {
						log.Errorf(ctx, "Failed to delete created file: %s", response.Err)
						return response
					}
					log.Infof(ctx, "Deleted file that was created by append operation: %s", filepath)
				}
				return spec.ReturnSuccess("File append destroy operation completed (deleted created file)")
			}

			// Restore the original file content and remove the backup file
			response := f.channel.Run(ctx, "cp", fmt.Sprintf(`"%s" "%s"`, backupFile, filepath))
			if !response.Success {
				log.Errorf(ctx, "Failed to restore original file content: %s", response.Err)
				return response
			}

			// Remove the backup file
			_ = f.channel.Run(ctx, "rm", fmt.Sprintf(`"%s"`, backupFile))

			log.Infof(ctx, "File append destroy operation completed for file: %s (original content restored)", filepath)
			return spec.ReturnSuccess("File append destroy operation completed successfully (original content restored)")
		} else {
			// If delete-file is true but enable-backup is false, delete the file
			if exec.CheckFilepathExists(ctx, f.channel, filepath) {
				response := f.channel.Run(ctx, "rm", fmt.Sprintf(`"%s"`, filepath))
				if !response.Success {
					log.Errorf(ctx, "Failed to delete file: %s, error: %s", filepath, response.Err)
					return response
				}
				log.Infof(ctx, "Deleted file that was created/modified by append operation: %s", filepath)
			} else {
				log.Infof(ctx, "File does not exist, nothing to delete: %s", filepath)
			}
			return spec.ReturnSuccess("File append destroy operation completed (file deleted)")
		}
	}

	// If delete-file is false, handle based on backup settings
	if !enableBackup {
		// If backup is disabled and delete-file is false, do nothing (keep the file)
		log.Infof(ctx, "File append destroy operation completed for file: %s (no action taken, file preserved)", filepath)
		return spec.ReturnSuccess("File append destroy operation completed (file preserved)")
	}

	// Get the experiment UID to find the backup file
	uid := ctx.Value(spec.Uid)
	if uid == nil || uid == spec.UnknownUid || uid == "" {
		log.Errorf(ctx, "Cannot get experiment UID for file append destroy")
		return spec.ReturnFail(spec.ParameterInvalid, "experiment UID is required for destroy operation")
	}

	// The backup file should be stored as .chaos-blade-backup-{uid}
	backupFile := filepath + ".chaos-blade-backup-" + uid.(string)

	// Check if backup file exists
	if !exec.CheckFilepathExists(ctx, f.channel, backupFile) {
		// If no backup file exists, it means the original file didn't exist
		// Since delete-file is false, we keep the file that was created by the append operation
		log.Infof(ctx, "No backup file exists, keeping created file: %s", filepath)
		return spec.ReturnSuccess("File append destroy operation completed (created file preserved)")
	}

	// Since delete-file is false, we keep both the appended file and the backup file
	// No restoration is performed - the file remains in its appended state
	log.Infof(ctx, "File append destroy operation completed for file: %s (appended file and backup preserved)", filepath)
	return spec.ReturnSuccess("File append destroy operation completed successfully (appended file and backup preserved)")
}

func (f *FileAppendActionExecutor) SetChannel(channel spec.Channel) {
	f.channel = channel
}

func appendFile(cl spec.Channel, count int, ctx context.Context, content string, filepath string, escape bool, enableBase64 bool) *spec.Response {
	var response *spec.Response

	// Check if the directory exists, if not create it
	dir := path.Dir(filepath)
	if !exec.CheckFilepathExists(ctx, cl, dir) {
		response = cl.Run(ctx, "mkdir", fmt.Sprintf(`-p "%s"`, dir))
		if !response.Success {
			log.Errorf(ctx, "Failed to create directory: %s, error: %s", dir, response.Err)
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("failed to create directory %s: %s", dir, response.Err))
		}
		log.Infof(ctx, "Created directory: %s", dir)
	}

	if enableBase64 {
		decodeBytes, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return spec.ReturnFail(spec.OsCmdExecFailed, fmt.Sprintf("%s base64 decode err", content))
		}
		content = string(decodeBytes)
	}
	content = parseDate(content)
	for i := 0; i < count; i++ {
		response = parseRandom(content)
		if !response.Success {
			return response
		}
		content = response.Result.(string)
		if escape {
			response = cl.Run(ctx, "echo", fmt.Sprintf(`-e '%s' >> %s`, content, filepath))
		} else {
			response = cl.Run(ctx, "echo", fmt.Sprintf(`'%s' >> %s`, content, filepath))
		}
	}
	return response
}

func parseDate(content string) string {
	reg := regexp.MustCompile(`\\?@\{(?s:DATE\:([^(@{})]*[^\\]))\}`)
	result := reg.FindAllStringSubmatch(content, -1)
	for _, text := range result {
		if strings.HasPrefix(text[0], "\\@") {
			content = strings.Replace(content, text[0], strings.Replace(text[0], "\\", "", 1), 1)
			continue
		}
		content = strings.Replace(content, text[0], "$(date \""+text[1]+"\")", 1)
	}
	return content
}

func parseRandom(content string) *spec.Response {
	reg := regexp.MustCompile(`\\?@\{(?s:RANDOM\:([0-9]+\-[0-9]+))\}`)
	result := reg.FindAllStringSubmatch(content, -1)
	for _, text := range result {
		if strings.HasPrefix(text[0], "\\@") {
			content = strings.Replace(content, text[0], strings.Replace(text[0], "\\", "", 1), 1)
			continue
		}
		split := strings.Split(text[1], "-")
		begin, err := strconv.Atoi(split[0])
		if err != nil {
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("%d illegal parameter", begin))
		}

		end, err := strconv.Atoi(split[1])
		if err != nil {
			return spec.ReturnFail(spec.ParameterIllegal, fmt.Sprintf("%d illegal parameter", end))
		}

		if end <= begin {
			return spec.ReturnFail(spec.ParameterIllegal, "run append file failed, begin must < end")
		}
		content = strings.Replace(content, text[0], strconv.Itoa(rand.Intn(end-begin)+begin), 1)
	}
	return spec.ReturnSuccess(content)
}
