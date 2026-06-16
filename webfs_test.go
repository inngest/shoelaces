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

package shoelaces

import (
	"io/fs"
	"testing"
)

func TestEmbeddedUIFilesExist(t *testing.T) {
	tests := []struct {
		name string
		fsys fs.FS
		path string
	}{
		{
			name: "template filesystem",
			fsys: TemplateFS(),
			path: "index.html",
		},
		{
			name: "static filesystem template path",
			fsys: StaticFS(),
			path: "templates/html/index.html",
		},
		{
			name: "static filesystem asset path",
			fsys: StaticFS(),
			path: "js/jquery.min.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fs.Stat(tt.fsys, tt.path)
			if err != nil {
				t.Fatalf("expected %s in embedded UI filesystem: %v", tt.path, err)
			}
			if info.IsDir() {
				t.Fatalf("expected %s to be a file", tt.path)
			}
			if info.Size() == 0 {
				t.Fatalf("expected %s to be non-empty", tt.path)
			}
		})
	}
}
