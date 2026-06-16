// Copyright 2018 ThousandEyes Inc.
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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMacColonToDash(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want string
	}{
		{
			name: "colon separated mac",
			mac:  "ff:ff:ff:ff:ff:ff",
			want: "ff-ff-ff-ff-ff-ff",
		},
		{
			name: "already dash separated mac",
			mac:  "ff-ff-ff-ff-ff-ff",
			want: "ff-ff-ff-ff-ff-ff",
		},
		{
			name: "dot separated mac is unchanged",
			mac:  "ff.ff.ff.ff.ff.ff",
			want: "ff.ff.ff.ff.ff.ff",
		},
		{
			name: "empty mac is unchanged",
			mac:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MacColonToDash(tt.mac))
		})
	}
}

func TestMacDashToColon(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want string
	}{
		{
			name: "dash separated mac",
			mac:  "ff-ff-ff-ff-ff-ff",
			want: "ff:ff:ff:ff:ff:ff",
		},
		{
			name: "already colon separated mac",
			mac:  "ff:ff:ff:ff:ff:ff",
			want: "ff:ff:ff:ff:ff:ff",
		},
		{
			name: "dot separated mac is unchanged",
			mac:  "ff.ff.ff.ff.ff.ff",
			want: "ff.ff.ff.ff.ff.ff",
		},
		{
			name: "empty mac is unchanged",
			mac:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MacDashToColon(tt.mac))
		})
	}
}
