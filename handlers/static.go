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
	"github.com/inngest/shoelaces/bootsession"
	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/log"
)

// StaticConfigFileHandler handles static config files
type StaticConfigFileHandler struct{}

func (s *StaticConfigFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)
	envName := envNameFromRequest(r)
	fp := cleanRequestPath(r.URL.Path)
	if isStaticTemplateSourcePath(env, fp) {
		env.Logger.Debug("Static template source request rejected", "component", "static", "path", fp)
		http.NotFound(w, r)
		return
	}
	templateName := path.Join("static", fp)

	var resolved resolvedTemplateRequest
	resolvedOK := false
	resolveTemplateContext := func() bool {
		if resolvedOK {
			return true
		}
		var ok bool
		resolved, ok = resolveTemplateRequest(w, r, env, templateName, false)
		if !ok {
			return false
		}
		resolvedOK = true
		return true
	}

	resolvedEnvName := envName
	if r.URL.Query().Get(bootsession.QueryParam) != "" {
		if !resolveTemplateContext() {
			return
		}
		resolvedEnvName = resolved.envName
	}

	diskLayers := staticConfigDiskLayers(env, resolvedEnvName)
	embeddedLayer, err := embeddedStaticConfigLayer()
	if err != nil {
		env.Logger.Error("Failed to initialize embedded static filesystem", "component", "static", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allLayers := append(append([]overlayLayer{}, diskLayers...), embeddedLayer)
	exists, isDir, err := overlayPathStatus(allLayers, fp)
	if err != nil {
		env.Logger.Error("Failed to inspect static overlay", "component", "static", "path", fp, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists && isDir {
		(&OverlayFileServerHandler{layers: allLayers}).WithLogger(env.Logger).ServeHTTP(w, r)
		return
	}

	envDiskLayers := staticConfigEnvDiskLayers(env, resolvedEnvName)
	exists, _, err = overlayPathStatus(envDiskLayers, fp)
	if err != nil {
		env.Logger.Error("Failed to inspect static disk overlay", "component", "static", "path", fp, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		(&OverlayFileServerHandler{layers: envDiskLayers}).WithLogger(env.Logger).ServeHTTP(w, r)
		return
	}

	// Static provisioning helpers may be authored as .slc templates while still
	// being fetched from /configs/static/<name>. Literal static files win above;
	// this fallback treats disk static templates as user overrides before
	// falling back to embedded literal static defaults.
	if env.Templates != nil && resolvedEnvName != "" && env.Templates.HasTemplateOverride(templateName, resolvedEnvName) {
		if !resolveTemplateContext() {
			return
		}
		configString, err := env.Templates.RenderTemplate(templateName, resolved.variables, resolved.envName)
		if err != nil {
			env.Logger.Error("Failed to render static config template", "component", "static", "template", templateName, "environment", resolved.envName, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, configString)
		return
	}

	baseDiskLayers := staticConfigBaseDiskLayers(env)
	exists, _, err = overlayPathStatus(baseDiskLayers, fp)
	if err != nil {
		env.Logger.Error("Failed to inspect static base disk overlay", "component", "static", "path", fp, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		(&OverlayFileServerHandler{layers: baseDiskLayers}).WithLogger(env.Logger).ServeHTTP(w, r)
		return
	}

	if env.Templates != nil && env.Templates.HasTemplate(templateName, "") {
		if !resolveTemplateContext() {
			return
		}
		configString, err := env.Templates.RenderTemplate(templateName, resolved.variables, resolved.envName)
		if err != nil {
			env.Logger.Error("Failed to render static config template", "component", "static", "template", templateName, "environment", resolved.envName, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, configString)
		return
	}

	embeddedLayers := []overlayLayer{embeddedLayer}
	exists, _, err = overlayPathStatus(embeddedLayers, fp)
	if err != nil {
		env.Logger.Error("Failed to inspect embedded static overlay", "component", "static", "path", fp, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		(&OverlayFileServerHandler{layers: embeddedLayers}).WithLogger(env.Logger).ServeHTTP(w, r)
		return
	}

	env.Logger.Debug("Static overlay file not found", "component", "static", "path", fp)
	http.NotFound(w, r)
}

func staticConfigDiskLayers(env *environment.Environment, envName string) []overlayLayer {
	return append(staticConfigEnvDiskLayers(env, envName), staticConfigBaseDiskLayers(env)...)
}

func staticConfigEnvDiskLayers(env *environment.Environment, envName string) []overlayLayer {
	if envName == "" {
		return nil
	}
	envPath := filepath.Join(env.DataDir, env.EnvDir, envName, "static")
	return []overlayLayer{{dir: envPath}}
}

func staticConfigBaseDiskLayers(env *environment.Environment) []overlayLayer {
	return []overlayLayer{{dir: path.Join(env.DataDir, "static")}}
}

func isStaticTemplateSourcePath(env *environment.Environment, fp string) bool {
	return env.TemplateExtension != "" && strings.HasSuffix(fp, env.TemplateExtension)
}

func embeddedStaticConfigLayer() (overlayLayer, error) {
	embeddedStatic, err := fs.Sub(shoelaces.ProvisioningDefaultsFS(), "static")
	if err != nil {
		return overlayLayer{}, err
	}
	return overlayLayer{fsys: embeddedStatic}, nil
}

func overlayLayersWithFS(lower fs.FS, diskDirs ...string) []overlayLayer {
	layers := make([]overlayLayer, 0, len(diskDirs)+1)
	for _, dir := range diskDirs {
		layers = append(layers, overlayLayer{dir: dir})
	}
	return append(layers, overlayLayer{fsys: lower})
}

func overlayPathStatus(layers []overlayLayer, fp string) (bool, bool, error) {
	for _, layer := range layers {
		file, info, err := layer.open(fp)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, false, err
		}
		_ = file.Close()
		return true, info.IsDir(), nil
	}
	return false, false, nil
}

// StaticConfigFileServer returns a StaticConfigFileHandler instance implementing http.Handler
func StaticConfigFileServer() *StaticConfigFileHandler {
	return &StaticConfigFileHandler{}
}

// OverlayFileServerHandler handles request for overlayer directories
type OverlayFileServerHandler struct {
	layers []overlayLayer
	logger log.Logger
}

type overlayLayer struct {
	dir  string
	fsys fs.FS
}

func (l overlayLayer) name() string {
	if l.fsys != nil {
		return "embedded"
	}
	return l.dir
}

// OverlayFileServer serves static content from two overlayed directories
func OverlayFileServer(upper, lower string) *OverlayFileServerHandler {
	return &OverlayFileServerHandler{
		layers: []overlayLayer{{dir: upper}, {dir: lower}},
		logger: log.MakeLogger(io.Discard).With("component", "static"),
	}
}

// OverlayFileServerWithFS serves static content from disk directories first and
// an embedded filesystem last.
func OverlayFileServerWithFS(lower fs.FS, diskDirs ...string) *OverlayFileServerHandler {
	return &OverlayFileServerHandler{layers: overlayLayersWithFS(lower, diskDirs...), logger: log.MakeLogger(io.Discard).With("component", "static")}
}

// WithLogger overrides the default discard logger used by standalone overlay
// servers.
func (o *OverlayFileServerHandler) WithLogger(logger log.Logger) *OverlayFileServerHandler {
	if logger == nil {
		logger = log.MakeLogger(io.Discard)
	}
	logger = logger.With("component", "static")
	o.logger = logger
	return o
}

func (o *OverlayFileServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fp := cleanRequestPath(r.URL.Path)
	o.logger.Debug("Serving static overlay request", "path", fp, "layers", len(o.layers))

	isDir := false
	fileList := make(map[string]os.FileInfo)
	var firstFile overlayFile

	for _, layer := range o.layers {
		file, info, err := layer.open(fp)
		if err != nil {
			if os.IsNotExist(err) {
				o.logger.Debug("Static overlay layer missed", "path", fp, "layer", layer.name())
				continue
			}
			o.logger.Error("Static overlay layer failed", "path", fp, "layer", layer.name(), "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		o.logger.Debug("Static overlay layer matched", "path", fp, "layer", layer.name(), "directory", info.IsDir())
		opened := overlayFile{file: file, info: info}
		if info.IsDir() {
			files, err := opened.readDir()
			_ = opened.close()
			if err != nil {
				o.logger.Error("Failed to read static overlay directory", "path", fp, "layer", layer.name(), "err", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, f := range files {
				if _, ok := fileList[f.Name()]; !ok {
					info, err := f.Info()
					if err != nil {
						o.logger.Error("Failed to inspect static overlay entry", "path", fp, "layer", layer.name(), "entry", f.Name(), "err", err)
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					fileList[f.Name()] = info
				}
			}
			o.logger.Debug("Merged static overlay directory", "path", fp, "layer", layer.name(), "entries", len(fileList))
			isDir = true
			continue
		}
		if firstFile.file == nil {
			firstFile = opened
			o.logger.Debug("Selected static overlay file", "path", fp, "layer", layer.name(), "file", info.Name())
		} else {
			_ = opened.close()
		}
	}

	if !isDir && firstFile.file == nil {
		o.logger.Debug("Static overlay file not found", "path", fp)
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

	defer func() { _ = firstFile.close() }()
	readSeeker, ok := firstFile.file.(io.ReadSeeker)
	if !ok {
		o.logger.Error("Static overlay file cannot be served", "path", fp, "file", firstFile.info.Name())
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
