/*
 * Copyright 1999-2019 Alibaba Group Holding Ltd.
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

package version

import (
	"fmt"
	"runtime"
	"time"
)

// 版本信息结构体
type VersionInfo struct {
	Version      string    `json:"version"`
	GitCommit    string    `json:"git_commit"`
	BuildTime    time.Time `json:"build_time"`
	GoVersion    string    `json:"go_version"`
	Platform     string    `json:"platform"`
	Architecture string    `json:"architecture"`
}

// 构建时注入的变量（通过 ldflags 设置）
var (
	// BladeVersion 是主版本号，通过构建时注入
	BladeVersion = "dev"

	// GitCommit 是 Git 提交哈希，通过构建时注入
	GitCommit = "unknown"

	// BuildTime 是构建时间，通过构建时注入
	BuildTime = "unknown"
)

// GetVersionInfo 返回完整的版本信息
func GetVersionInfo() *VersionInfo {
	buildTime, _ := time.Parse(time.RFC3339, BuildTime)
	if buildTime.IsZero() {
		buildTime = time.Now()
	}

	return &VersionInfo{
		Version:      BladeVersion,
		GitCommit:    GitCommit,
		BuildTime:    buildTime,
		GoVersion:    runtime.Version(),
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}

// GetVersion 返回版本字符串
func GetVersion() string {
	return BladeVersion
}

// GetFullVersion 返回完整版本信息字符串
func GetFullVersion() string {
	info := GetVersionInfo()
	return fmt.Sprintf("chaosblade-exec-os %s (commit: %s, built: %s, go: %s, %s/%s)",
		info.Version,
		info.GitCommit[:8], // 只显示前8位
		info.BuildTime.Format("2006-01-02 15:04:05"),
		info.GoVersion,
		info.Platform,
		info.Architecture,
	)
}

// IsRelease 判断是否为发布版本
func IsRelease() bool {
	return BladeVersion != "dev" && BladeVersion != "latest"
}

// GetShortCommit 返回短提交哈希
func GetShortCommit() string {
	if len(GitCommit) >= 8 {
		return GitCommit[:8]
	}
	return GitCommit
}
