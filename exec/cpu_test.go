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

package exec

import (
	"testing"

	"github.com/chaosblade-io/chaosblade-spec-go/util"
)

func TestParseCpuList(t *testing.T) {
	tests := []struct {
		input  string
		expect []string
	}{
		{"0-3", []string{"0", "1", "2", "3"}},
		{"1,3,5", []string{"1", "3", "5"}},
		{"0-2,4,6-7", []string{"0", "1", "2", "4", "6", "7"}},
	}
	for _, tt := range tests {
		got, err := util.ParseIntegerListToStringSlice(tt.input)
		if err != nil {
			t.Errorf("input is illegal")
		}
		if len(got) != len(tt.expect) {
			t.Errorf("expected to see %d cpu, got %d", len(tt.expect), len(got))
		}
		for i, m := range tt.expect {
			if got[i] != m {
				t.Errorf("unexpected result: %s, expected: %s", got, tt.expect)
			}
		}
	}
}
