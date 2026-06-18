// Copyright 2026 ThousandEyes Inc.
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

package router

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/handlers"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestConfigsStaticRouteServesEmbeddedProvisioningAsset(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/configs/static/provisioning-default.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "generic embedded provisioning static asset")
}

func TestConfigsStaticRouteDiskOverridesEmbeddedProvisioningAsset(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "static"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "static", "provisioning-default.txt"), []byte("from disk\n"), 0o644))
	handler := newTestRouter(t, dataDir)
	req := httptest.NewRequest(http.MethodGet, "/configs/static/provisioning-default.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "from disk\n", rr.Body.String())
}

func TestConfigsStaticRouteMissingFileReturnsNotFound(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/configs/static/missing.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestStaticRouteDoesNotServeProvisioningDefaults(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/static/provisioning-default.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestConfigTemplateRouteUsesQueryParamsWithoutMappingDefaults(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "install.ipxe.slc"), []byte(`{{define "install.ipxe"}}release={{.release}}
baseURL={{.baseURL}}
{{end}}
`), 0o644))

	handler := newTestRouterWithEnvironment(t, dataDir, func(env *environment.Environment) {
		env.Templates = templates.New(env.Logger)
		env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
		env.MappingResolver = mustMappingResolver(t, &mappings.Mappings{
			Defaults: mappings.DefaultsMap{
				Params: map[string]interface{}{
					"release": "mapping-release",
					"secret":  "mapping-secret",
				},
			},
			Targets: map[string]mappings.Target{
				"debian12": {Script: "install.ipxe"},
			},
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/configs/install.ipxe?release=query-release", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "release=query-release")
	assert.Contains(t, rr.Body.String(), "baseURL=localhost:8081")
	assert.NotContains(t, rr.Body.String(), "mapping-release")
	assert.NotContains(t, rr.Body.String(), "mapping-secret")
}

func TestConfigTemplateRouteRendersEmbeddedProvisioningTemplate(t *testing.T) {
	dataDir := t.TempDir()
	handler := newTestRouterWithEnvironment(t, dataDir, func(env *environment.Environment) {
		env.Templates = templates.New(env.Logger)
		env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
	})
	req := httptest.NewRequest(http.MethodGet, "/configs/preseed/debian?encrypt_home=false", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "d-i user-setup/encrypt-home boolean false")
	assert.Contains(t, rr.Body.String(), "d-i finish-install/reboot_in_progress note")
	assert.NotContains(t, rr.Body.String(), "d-i preseed/late_command")
}

func TestConfigTemplateRoutePreservesStructuredProvisioningFromBootURL(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "provisioning"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "provisioning", "extra.slc"), []byte(`{{define "provisioning/extra" -}}
d-i preseed/late_command string echo extra {{.hostname}} {{.storage_disk}}
{{end}}
`), 0o644))

	var bootScript string
	handler := newTestRouterWithEnvironment(t, dataDir, func(env *environment.Environment) {
		env.Templates = templates.New(env.Logger)
		env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
		params := mappings.ParamsWithProvisioning(map[string]interface{}{
			"baseURL":  env.BaseURL,
			"hostname": "boot-host",
		}, map[string]mappings.ResolvedUser{
			"infra": {
				Name:            "infra",
				Primary:         true,
				FullName:        "Infrastructure User",
				PasswordCrypted: "$6$infra",
			},
		}, mappings.ProvisioningConfig{
			Packages: mappings.PackagesConfig{
				Install: []string{"curl", "vim"},
			},
			Storage: mappings.StorageConfig{
				Disk: "/dev/vda",
			},
			Repos: mappings.ReposConfig{
				Release: "bookworm",
			},
			Installer: mappings.InstallerConfig{
				ConfigTemplate: "preseed/debian",
				ExtraTemplate:  "provisioning/extra",
				ConfigParams: map[string]any{
					"encrypt_home": false,
				},
			},
		})

		var err error
		bootScript, err = env.Templates.RenderTemplate("debian.ipxe", params, "")
		require.NoError(t, err)
	})
	preseedURL := renderedPreseedURL(t, bootScript)
	req := httptest.NewRequest(http.MethodGet, preseedURL.RequestURI(), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	assert.Contains(t, rr.Body.String(), "d-i partman-auto/disk string /dev/vda")
	assert.Contains(t, rr.Body.String(), "d-i pkgsel/include string curl vim")
	assert.Contains(t, rr.Body.String(), "d-i passwd/user-fullname string Infrastructure User")
	assert.Contains(t, rr.Body.String(), "d-i passwd/username string infra")
	assert.Contains(t, rr.Body.String(), "d-i preseed/late_command string echo extra boot-host /dev/vda")
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

func mustMappingResolver(t *testing.T, config *mappings.Mappings) *mappings.Resolver {
	t.Helper()

	resolver, err := mappings.NewResolver(config)
	require.NoError(t, err)
	return resolver
}

func renderedPreseedURL(t *testing.T, bootScript string) *url.URL {
	t.Helper()

	for _, field := range strings.Fields(bootScript) {
		if strings.HasPrefix(field, "preseed/url=") {
			parsed, err := url.Parse(strings.TrimPrefix(field, "preseed/url="))
			require.NoError(t, err)
			return parsed
		}
	}
	t.Fatalf("rendered boot script did not contain preseed/url: %s", bootScript)
	return nil
}
