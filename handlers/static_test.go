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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverlayFileServerServesFiles(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		setup     func(t *testing.T, upper, lower string)
		wantBody  string
		wantCode  int
		wantFound bool
	}{
		{
			name: "upper layer wins over lower layer",
			path: "/shared.txt",
			setup: func(t *testing.T, upper, lower string) {
				writeTestFile(t, filepath.Join(lower, "shared.txt"), "from lower")
				writeTestFile(t, filepath.Join(upper, "shared.txt"), "from upper")
			},
			wantBody:  "from upper",
			wantCode:  http.StatusOK,
			wantFound: true,
		},
		{
			name: "lower layer serves fallback file",
			path: "/lower-only.txt",
			setup: func(t *testing.T, upper, lower string) {
				writeTestFile(t, filepath.Join(lower, "lower-only.txt"), "from lower")
			},
			wantBody:  "from lower",
			wantCode:  http.StatusOK,
			wantFound: true,
		},
		{
			name:      "missing file returns not found",
			path:      "/missing.txt",
			setup:     func(t *testing.T, upper, lower string) {},
			wantCode:  http.StatusNotFound,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upper := t.TempDir()
			lower := t.TempDir()
			tt.setup(t, upper, lower)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			OverlayFileServer(upper, lower).ServeHTTP(rr, req)

			require.Equal(t, tt.wantCode, rr.Code)
			if tt.wantFound {
				assert.True(t, strings.HasPrefix(rr.Body.String(), tt.wantBody), "expected %q prefix, got %q", tt.wantBody, rr.Body.String())
			}
		})
	}
}

func TestOverlayFileServerMergesDirectoryIndex(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	writeTestFile(t, filepath.Join(lower, "b.txt"), "lower")
	writeTestFile(t, filepath.Join(upper, "a.txt"), "upper")
	writeTestFile(t, filepath.Join(upper, "b.txt"), "upper wins")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `<a href="a.txt">a.txt</a>`)
	assert.Equal(t, 1, strings.Count(body, `<a href="b.txt">b.txt</a>`))
	assert.Less(t, strings.Index(body, "a.txt"), strings.Index(body, "b.txt"))
}

func TestOverlayFileServerIncludesLowerLayerDirectoryEntries(t *testing.T) {
	upper := t.TempDir()
	lower := t.TempDir()

	require.NoError(t, os.Mkdir(filepath.Join(lower, "lower-dir"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(upper, "upper-dir"), 0o755))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	OverlayFileServer(upper, lower).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `<a href="lower-dir/">lower-dir/</a>`)
	assert.Contains(t, body, `<a href="upper-dir/">upper-dir/</a>`)
}

func TestOverlayFileServerWithFSMergesDirectoryIndex(t *testing.T) {
	upper := t.TempDir()
	writeTestFile(t, filepath.Join(upper, "disk.txt"), "from disk")
	lower := fstest.MapFS{
		"embedded.txt": {Data: []byte("from embedded")},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	OverlayFileServerWithFS(lower, upper).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `<a href="disk.txt">disk.txt</a>`)
	assert.Contains(t, body, `<a href="embedded.txt">embedded.txt</a>`)
}

func TestStaticConfigFileServerDoesNotServeTemplateSource(t *testing.T) {
	dataDir := t.TempDir()
	writeTestFile(t, filepath.Join(dataDir, "static", "install-firstboot.sh.slc"), `{{define "static/install-firstboot.sh" -}}
#!/bin/sh
echo rendered
{{end}}
`)
	env := newStaticTestEnvironment(t, dataDir)

	rr := httptest.NewRecorder()
	req := newStaticTestRequest("/install-firstboot.sh", env)
	StaticConfigFileServer().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "echo rendered")

	rr = httptest.NewRecorder()
	req = newStaticTestRequest("/install-firstboot.sh.slc", env)
	StaticConfigFileServer().ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	assert.NotContains(t, rr.Body.String(), `{{define "static/install-firstboot.sh"`)
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func newStaticTestEnvironment(t *testing.T, dataDir string) *environment.Environment {
	t.Helper()

	logger := log.MakeLogger(io.Discard)
	renderer := templates.New(logger)
	renderer.ParseTemplates(dataDir, "env_overrides", nil, ".slc")
	return &environment.Environment{
		DataDir:           dataDir,
		EnvDir:            "env_overrides",
		TemplateExtension: ".slc",
		BaseURL:           "http://shoelaces.example.test",
		Logger:            logger,
		Templates:         renderer,
	}
}

func newStaticTestRequest(path string, env *environment.Environment) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := context.WithValue(req.Context(), ShoelacesEnvCtxID, env)
	return req.WithContext(ctx)
}
