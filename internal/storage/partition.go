// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PartitionPath returns the Linux block-device path for a partition on device.
// Debian preseed's partman-auto-raid recipe needs explicit member partitions;
// using raidid=<n> was treated as a device path in live d-i testing.
func PartitionPath(device string, number int) string {
	if strings.HasPrefix(device, "/dev/disk/") {
		return fmt.Sprintf("%s-part%d", device, number)
	}
	if device != "" {
		last, _ := utf8.DecodeLastRuneInString(device)
		if unicode.IsDigit(last) {
			return fmt.Sprintf("%sp%d", device, number)
		}
	}
	return fmt.Sprintf("%s%d", device, number)
}
