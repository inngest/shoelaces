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
	"embed"
	"io/fs"
)

//go:embed web
var embeddedWeb embed.FS

// StaticFS returns the embedded UI filesystem rooted like the historical web
// directory, so paths such as js/jquery.min.js and css/bootstrap.min.css resolve.
func StaticFS() fs.FS {
	return mustSub(embeddedWeb, "web")
}

// TemplateFS returns the embedded UI HTML template filesystem.
func TemplateFS() fs.FS {
	return mustSub(embeddedWeb, "web/templates/html")
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
