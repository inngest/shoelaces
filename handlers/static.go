// Copyright 2018 ThousandEyes Inc.
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

package handlers

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	shoelaces "github.com/inngest/shoelaces"
)

// StaticConfigFileHandler handles static config files
type StaticConfigFileHandler struct{}

func (s *StaticConfigFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)
	envName := envNameFromRequest(r)
	basePath := path.Join(env.DataDir, "static")
	embeddedStatic, err := fs.Sub(shoelaces.ProvisioningDefaultsFS(), "static")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if envName == "" {
		OverlayFileServerWithFS(embeddedStatic, basePath).ServeHTTP(w, r)
		return
	}
	envPath := filepath.Join(env.DataDir, env.EnvDir, envName, "static")
	OverlayFileServerWithFS(embeddedStatic, envPath, basePath).ServeHTTP(w, r)
}

// StaticConfigFileServer returns a StaticConfigFileHandler instance implementing http.Handler
func StaticConfigFileServer() *StaticConfigFileHandler {
	return &StaticConfigFileHandler{}
}

// OverlayFileServerHandler handles request for overlayer directories
type OverlayFileServerHandler struct {
	layers []overlayLayer
}

type overlayLayer struct {
	dir  string
	fsys fs.FS
}

// OverlayFileServer serves static content from two overlayed directories
func OverlayFileServer(upper, lower string) *OverlayFileServerHandler {
	return &OverlayFileServerHandler{
		layers: []overlayLayer{{dir: upper}, {dir: lower}},
	}
}

// OverlayFileServerWithFS serves static content from disk directories first and
// an embedded filesystem last.
func OverlayFileServerWithFS(lower fs.FS, diskDirs ...string) *OverlayFileServerHandler {
	layers := make([]overlayLayer, 0, len(diskDirs)+1)
	for _, dir := range diskDirs {
		layers = append(layers, overlayLayer{dir: dir})
	}
	layers = append(layers, overlayLayer{fsys: lower})
	return &OverlayFileServerHandler{layers: layers}
}

func (o *OverlayFileServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fp := cleanRequestPath(r.URL.Path)

	isDir := false
	fileList := make(map[string]os.FileInfo)
	var firstFile overlayFile

	for _, layer := range o.layers {
		file, info, err := layer.open(fp)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		opened := overlayFile{file: file, info: info}
		if info.IsDir() {
			files, err := opened.readDir()
			_ = opened.close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, f := range files {
				if _, ok := fileList[f.Name()]; !ok {
					info, err := f.Info()
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					fileList[f.Name()] = info
				}
			}
			isDir = true
			continue
		}
		if firstFile.file == nil {
			firstFile = opened
		} else {
			_ = opened.close()
		}
	}

	if !isDir && firstFile.file == nil {
		http.NotFound(w, r)
		return
	}

	if isDir {
		if firstFile.file != nil {
			_ = firstFile.close()
		}
		writeDirectoryIndex(w, fileList)
		return
	}

	defer firstFile.close()
	readSeeker, ok := firstFile.file.(io.ReadSeeker)
	if !ok {
		http.Error(w, fmt.Sprintf("file %q cannot be served", firstFile.info.Name()), http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, firstFile.info.Name(), firstFile.info.ModTime(), readSeeker)
}

type overlayFile struct {
	file fs.File
	info os.FileInfo
}

func (f overlayFile) close() error {
	return f.file.Close()
}

func (l overlayLayer) open(fp string) (fs.File, os.FileInfo, error) {
	if l.fsys != nil {
		file, err := l.fsys.Open(fp)
		if err != nil {
			return nil, nil, err
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, nil, err
		}
		return file, info, nil
	}

	file, err := os.Open(filepath.Clean(path.Join(l.dir, fp)))
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

func (f overlayFile) readDir() ([]fs.DirEntry, error) {
	dir, ok := f.file.(fs.ReadDirFile)
	if !ok {
		return nil, fmt.Errorf("directory %q cannot be listed", f.info.Name())
	}
	return dir.ReadDir(-1)
}

func cleanRequestPath(requestPath string) string {
	cleaned := path.Clean("/" + requestPath)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func writeDirectoryIndex(w http.ResponseWriter, fileList map[string]os.FileInfo) {
	fileListIndex := []string{}
	for i := range fileList {
		fileListIndex = append(fileListIndex, i)
	}
	sort.Strings(fileListIndex)
	_, _ = w.Write([]byte("<pre>\n"))
	for _, i := range fileListIndex {
		f := fileList[i]
		name := f.Name()
		if f.IsDir() {
			name = name + "/"
		}
		l := fmt.Sprintf("<a href=\"%s\">%s</a>\n", name, name)
		_, _ = w.Write([]byte(l))
	}
	_, _ = w.Write([]byte("</pre>\n"))
}
