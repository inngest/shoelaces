// Copyright 2026 ThousandEyes Inc.
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

package router

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	shoelaces "github.com/thousandeyes/shoelaces"
	"github.com/thousandeyes/shoelaces/internal/environment"
	"github.com/thousandeyes/shoelaces/internal/handlers"
	"github.com/thousandeyes/shoelaces/internal/log"
)

func TestUIRoutesRenderEmbeddedTemplates(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())

	for _, path := range []string{"/", "/events", "/mappings"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Body.String(), "shoelaces - painless server bootstrapping")
		})
	}
}

func TestStaticRouteServesEmbeddedUIAsset(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/static/js/jquery.min.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "jQuery")
}

func TestStaticRouteServesUIDirOverrideAsset(t *testing.T) {
	uiDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(uiDir, "js"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(uiDir, "js", "jquery.min.js"), []byte("from ui dir\n"), 0o644))
	handler := newTestRouterWithEnvironment(t, t.TempDir(), func(env *environment.Environment) {
		env.UIDir = uiDir
		env.UIOverrideDirSet = true
	})
	req := httptest.NewRequest(http.MethodGet, "/static/js/jquery.min.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "from ui dir\n", rr.Body.String())
}

func TestConfigsStaticRouteServesDataDirStaticFiles(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "static"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "static", "provision.txt"), []byte("from data dir\n"), 0o644))
	handler := newTestRouter(t, dataDir)
	req := httptest.NewRequest(http.MethodGet, "/configs/static/provision.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "from data dir\n", rr.Body.String())
}

func newTestRouter(t *testing.T, dataDir string) http.Handler {
	t.Helper()

	return newTestRouterWithEnvironment(t, dataDir, nil)
}

func newTestRouterWithEnvironment(t *testing.T, dataDir string, configure func(*environment.Environment)) http.Handler {
	t.Helper()

	staticTemplates := template.Must(template.ParseFS(shoelaces.TemplateFS(),
		"header.html",
		"index.html",
		"events.html",
		"mappings.html",
		"footer.html",
	))
	env := &environment.Environment{
		BaseURL:           "localhost:8081",
		DataDir:           dataDir,
		EnvDir:            "env_overrides",
		UIDir:             filepath.Join(t.TempDir(), "missing-web"),
		TemplateExtension: ".slc",
		StaticTemplates:   staticTemplates,
		Logger:            log.MakeLogger(io.Discard),
	}
	if configure != nil {
		configure(env)
	}
	return handlers.MiddlewareChain(env).Then(ShoelacesRouter(env))
}
