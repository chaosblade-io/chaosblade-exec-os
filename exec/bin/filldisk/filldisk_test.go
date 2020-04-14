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
	"fmt"
	"math"
	"reflect"
	"strconv"
	"syscall"
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
)

func Test_startFill(t *testing.T) {
	type args struct {
		directory    string
		size         string
		percent      string
		reserve      string
		retainHandle bool
	}
	tests := []struct {
		name   string
		args   args
		err    error
		result string
	}{
		{
			name: "directory value is empty",
			args: args{"", "", "", "", false},
			err:  fmt.Errorf("--directory flag value is empty"),
		},
		{
			name: "size,percent and reserve values are empty",
			args: args{"/dev", "", "", "", false},
			err:  fmt.Errorf("less --size or --percent or --reserve flag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err, result := startFill(tt.args.directory, tt.args.size, tt.args.percent, tt.args.reserve, tt.args.retainHandle)
			if !reflect.DeepEqual(err, tt.err) {
				t.Errorf("startFill() got = %v, want %v", err, tt.err)
			}
			if result != tt.result {
				t.Errorf("startFill() got1 = %v, want %v", result, tt.result)
			}
		})
	}
}

func Test_calculateFileSize(t *testing.T) {
	// mock sys stat
	getSysStatFunc = func(directory string) *syscall.Statfs_t {
		return &syscall.Statfs_t{
			Blocks: 10287952,
			Bavail: 6735107,
			Bsize:  4096,
			Bfree:  7263465,
		}
	}
	allBytes := 10287952 * 4096
	availableBytes := 6735107 * 4096
	usedBytes := allBytes - availableBytes
	usedPercentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(usedBytes)/float64(allBytes)), 64)
	expectedPercentage, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(50)/100.0), 64)
	remainderPercentage := expectedPercentage - usedPercentage
	expectSizeForPercent := math.Floor(remainderPercentage * float64(allBytes) / (1024.0 * 1024.0))

	availableMB := float64(availableBytes) / (1024.0 * 1024.0)
	expectSizeForReserve := math.Floor(availableMB - 1024)

	type args struct {
		directory string
		size      string
		percent   string
		reserve   string
	}
	tests := []struct {
		name         string
		args         args
		expectedSize string
		wantErr      bool
	}{
		{
			name: "size is 40G, percent is empty, reserve is empty",
			args: args{
				directory: "/",
				size:      "40960",
				percent:   "",
				reserve:   "",
			},
			expectedSize: "40960",
			wantErr:      false,
		},
		{
			name: "size is empty, percent is 50, reserve is 1024",
			args: args{
				directory: "/",
				size:      "",
				percent:   "50",
				reserve:   "1024",
			},
			expectedSize: fmt.Sprintf("%.f", expectSizeForPercent),
			wantErr:      false,
		},
		{
			name: "size is empty, percent is 20 less than used, reserve is 1024",
			args: args{
				directory: "/",
				size:      "",
				percent:   "20",
				reserve:   "1024",
			},
			expectedSize: "",
			wantErr:      true,
		},
		{
			name: "size is empty, percent is empty, reserve is 1024",
			args: args{
				directory: "/",
				size:      "",
				percent:   "",
				reserve:   "1024",
			},
			expectedSize: fmt.Sprintf("%.f", expectSizeForReserve),
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedSize, err := calculateFileSize(tt.args.directory, tt.args.size, tt.args.percent, tt.args.reserve)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateFileSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if expectedSize != tt.expectedSize {
				t.Errorf("calculateFileSize() got = %v, want %v", expectedSize, tt.expectedSize)
			}
		})
	}
}

func Test_stopFill(t *testing.T) {
	processMap := map[string]struct{}{
		"1": {},
		"2": {},
	}
	cl = &channel.MockLocalChannel{
		RunFunc: func(ctx context.Context, script, args string) *spec.Response {
			if script == "kill" {
				if args == "-9 1" {
					delete(processMap, "1")
				}
				if args == "-9 2" {
					delete(processMap, "2")
				}
			} else {
				t.Errorf("stopFill() error, not invoking kill command")
			}
			return nil
		},
		GetPidsByProcessNameFunc: func(processName string, ctx context.Context) ([]string, error) {
			if processName == fillDataFile {
				return []string{"1"}, nil
			}
			if processName == "retain-nohup" {
				return []string{"2"}, nil
			}
			return []string{}, nil
		},
	}
	type args struct {
		directory string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "directory is empty",
			args:    args{""},
			wantErr: true,
		},
		{
			name:    "directory is /",
			args:    args{"/"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := stopFill(tt.args.directory); (err != nil) != tt.wantErr {
				t.Errorf("stopFill() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	if len(processMap) != 0 {
		t.Errorf("stopFill() error, does not kill necessary process")
	}
}
