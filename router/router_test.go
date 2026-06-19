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
	"context"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/bootsession"
	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/handlers"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/persistence"
	"github.com/inngest/shoelaces/persistence/memory"
	"github.com/inngest/shoelaces/polling"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/templates"
	"github.com/oklog/ulid/v2"
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

func TestAjaxEventsReturnsGroupedEventShape(t *testing.T) {
	var eventLog *event.Log
	handler := newTestRouterWithEnvironment(t, t.TempDir(), func(env *environment.Environment) {
		eventLog = env.EventLog
	})
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "test-host")
	require.NoError(t, eventLog.AppendEvent(context.Background(), event.HostBoot, srv, event.SubnetMatchBoot, "debian.ipxe", map[string]any{"role": "web"}))
	req := httptest.NewRequest(http.MethodGet, "/ajax/events", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var events map[string][]event.Event
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &events))
	require.Len(t, events[srv.Mac], 1)
	assert.Equal(t, event.HostBoot, events[srv.Mac][0].Type)
	assert.Equal(t, "debian.ipxe", events[srv.Mac][0].Script)
	assert.Equal(t, "web", events[srv.Mac][0].Params["role"])
}

func TestAjaxEventReturnsSingleRedactedEvent(t *testing.T) {
	var eventLog *event.Log
	handler := newTestRouterWithEnvironment(t, t.TempDir(), func(env *environment.Environment) {
		eventLog = env.EventLog
	})
	srv := server.New("06:66:de:ad:be:ef", "192.0.2.10", "test-host")
	require.NoError(t, eventLog.AppendEvent(context.Background(), event.HostBoot, srv, event.SubnetMatchBoot, "debian.ipxe", map[string]any{
		"hostname":        "test-host",
		"bootstrap_token": "secret-token",
	}))
	events, err := eventLog.ListEvents(context.Background())
	require.NoError(t, err)
	id := events[srv.Mac][0].ID.String()
	req := httptest.NewRequest(http.MethodGet, "/ajax/events/"+id, nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got event.Event
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, id, got.ID.String())
	assert.Equal(t, "debian.ipxe", got.Script)
	assert.Equal(t, "test-host", got.Params["hostname"])
	assert.Equal(t, "[REDACTED]", got.Params["bootstrap_token"])
	assert.NotContains(t, rr.Body.String(), "secret-token")
}

func TestAjaxEventReturnsLookupErrors(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())

	for _, tt := range []struct {
		name   string
		id     string
		status int
		body   string
	}{
		{name: "invalid", id: "not-a-ulid", status: http.StatusBadRequest, body: "invalid event id"},
		{name: "missing", id: ulid.Make().String(), status: http.StatusNotFound, body: "event not found"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ajax/events/"+tt.id, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.status, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.body)
		})
	}
}

func TestAjaxBootSessionReferenceReturnsRedactedMetadata(t *testing.T) {
	var ref string
	handler := newTestRouterWithEnvironment(t, t.TempDir(), func(env *environment.Environment) {
		var err error
		ref, err = env.BootSessions.Create(context.Background(), bootsession.Snapshot{
			Server: server.New("06:66:de:ad:be:ef", "192.0.2.10", "boot-host"),
			Target: "debian.ipxe",
			Params: map[string]any{
				"hostname":        "boot-host",
				"bootstrap_token": "secret-token",
			},
			Users: map[string]mappings.ResolvedUser{
				"infra": {
					Name:            "infra",
					Primary:         true,
					PasswordCrypted: "$6$secret",
				},
			},
			Provisioning: mappings.ProvisioningConfig{
				Storage: mappings.StorageConfig{
					Encryption: mappings.StorageEncryptionConfig{
						Enabled:    boolPtr(true),
						Passphrase: "luks-passphrase",
					},
				},
				Installer: mappings.InstallerConfig{
					ConfigParams: map[string]any{"bootstrap_token": "secret-token"},
				},
			},
		})
		require.NoError(t, err)
	})
	req := httptest.NewRequest(http.MethodGet, "/ajax/boot-sessions/"+url.PathEscape(ref), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, ref, got["ref"])
	assert.Equal(t, "debian.ipxe", got["target"])
	params := got["params"].(map[string]any)
	assert.Equal(t, "boot-host", params["hostname"])
	assert.Equal(t, "[REDACTED]", params["bootstrap_token"])
	provisioning := got["provisioning"].(map[string]any)
	storage := provisioning["Storage"].(map[string]any)
	encryption := storage["Encryption"].(map[string]any)
	assert.Equal(t, "[REDACTED]", encryption["Passphrase"])
	assert.NotContains(t, rr.Body.String(), "secret-token")
	assert.NotContains(t, rr.Body.String(), "$6$secret")
	assert.NotContains(t, rr.Body.String(), "luks-passphrase")
}

func TestAjaxBootSessionReferenceReturnsNotFound(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/ajax/boot-sessions/missing-ref", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "boot reference not found")
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
				Params: map[string]any{
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
		users := map[string]mappings.ResolvedUser{
			"infra": {
				Name:            "infra",
				Primary:         true,
				FullName:        "Infrastructure User",
				PasswordCrypted: "$6$infra",
			},
		}
		provisioning := mappings.ProvisioningConfig{
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
		}
		params := mappings.ParamsWithProvisioning(map[string]any{
			"baseURL":  env.BaseURL,
			"hostname": "boot-host",
		}, users, provisioning)

		var err error
		ref, err := env.BootSessions.Create(context.Background(), bootsession.Snapshot{
			Server:       server.New("06:66:de:ad:be:ef", "192.0.2.10", "boot-host"),
			Target:       "debian.ipxe",
			Params:       params,
			Users:        users,
			Provisioning: provisioning,
		})
		require.NoError(t, err)
		bootsession.ApplyReferenceParams(params, ref)
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

func TestConfigTemplateRouteKeepsLUKSPassphraseBehindBootReference(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "provisioning"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "provisioning", "extra.slc"), []byte(`{{define "provisioning/extra" -}}
d-i preseed/late_command string echo luks {{.provisioning.Storage.Encryption.Passphrase}}
{{end}}
`), 0o644))

	var bootScript string
	handler := newTestRouterWithEnvironment(t, dataDir, func(env *environment.Environment) {
		env.Templates = templates.New(env.Logger)
		env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
		provisioning := mappings.ProvisioningConfig{
			Storage: mappings.StorageConfig{
				Disk: "/dev/vda",
				Encryption: mappings.StorageEncryptionConfig{
					Enabled:    boolPtr(true),
					Passphrase: "luks-passphrase",
				},
			},
			Repos: mappings.ReposConfig{
				Release: "trixie",
			},
			Installer: mappings.InstallerConfig{
				ConfigTemplate: "preseed/debian",
				ExtraTemplate:  "provisioning/extra",
				ConfigParams: map[string]any{
					"encrypt_home": false,
				},
			},
		}
		params := mappings.ParamsWithProvisioning(map[string]any{
			"baseURL":  env.BaseURL,
			"hostname": "boot-host",
		}, nil, provisioning)

		var err error
		ref, err := env.BootSessions.Create(context.Background(), bootsession.Snapshot{
			Server:       server.New("06:66:de:ad:be:ef", "192.0.2.10", "boot-host"),
			Target:       "debian.ipxe",
			Params:       params,
			Provisioning: provisioning,
		})
		require.NoError(t, err)
		bootsession.ApplyReferenceParams(params, ref)
		bootScript, err = env.Templates.RenderTemplate("debian.ipxe", params, "")
		require.NoError(t, err)
	})
	assert.Contains(t, bootScript, "ref=")
	assert.NotContains(t, bootScript, "luks-passphrase")
	assert.NotContains(t, bootScript, "provisioning=")

	preseedURL := renderedPreseedURL(t, bootScript)
	req := httptest.NewRequest(http.MethodGet, preseedURL.RequestURI(), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "d-i preseed/late_command string echo luks luks-passphrase")
}

func TestConfigTemplateRouteResolvesBootReferenceAndPreservesQueryOverrides(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "install.cfg.slc"), []byte(`{{define "install.cfg" -}}
hostname={{.hostname}}
disk={{.storage_disk}}
wipe={{.storage_wipe_disks}}
template_disk={{.storage_template_disk}}
kickstart_drive={{.kickstart_storage_drive}}
release={{.release}}
{{with $users := index . "users"}}{{with $users.Primary}}user={{.Name}}
{{end}}{{end}}
{{end}}
`), 0o644))

	var ref string
	handler := newTestRouterWithEnvironment(t, dataDir, func(env *environment.Environment) {
		env.Templates = templates.New(env.Logger)
		env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
		var err error
		ref, err = env.BootSessions.Create(context.Background(), bootsession.Snapshot{
			Server: server.New("06:66:de:ad:be:ef", "192.0.2.10", "boot-host"),
			Target: "install.cfg",
			Params: map[string]any{
				"hostname": "boot-host",
			},
			Users: map[string]mappings.ResolvedUser{
				"infra": {Name: "infra", Primary: true},
			},
			Provisioning: mappings.ProvisioningConfig{
				Repos: mappings.ReposConfig{Release: "trixie"},
				Storage: mappings.StorageConfig{
					Disk: "/dev/session",
				},
			},
		})
		require.NoError(t, err)
	})
	req := httptest.NewRequest(http.MethodGet, "/configs/install.cfg?ref="+url.QueryEscape(ref)+"&storage_disk=/dev/query", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "hostname=boot-host")
	assert.Contains(t, rr.Body.String(), "disk=/dev/query")
	assert.Contains(t, rr.Body.String(), "wipe=/dev/query")
	assert.Contains(t, rr.Body.String(), "template_disk=/dev/query")
	assert.Contains(t, rr.Body.String(), "kickstart_drive=query")
	assert.Contains(t, rr.Body.String(), "release=trixie")
	assert.Contains(t, rr.Body.String(), "user=infra")
}

func TestConfigTemplateRouteUsesBootReferenceEnvironmentWithoutEnvPath(t *testing.T) {
	dataDir := t.TempDir()
	envDir := filepath.Join(dataDir, "env_overrides", "staging")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "install.cfg.slc"), []byte(`{{define "install.cfg" -}}
variant=default
baseURL={{.baseURL}}
hostname={{.hostname}}
{{end}}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "install.cfg.slc"), []byte(`{{define "install.cfg" -}}
variant=staging
baseURL={{.baseURL}}
hostname={{.hostname}}
{{end}}
`), 0o644))

	var ref string
	handler := newTestRouterWithEnvironment(t, dataDir, func(env *environment.Environment) {
		env.Environments = []string{"staging"}
		env.Templates = templates.New(env.Logger)
		env.Templates.ParseTemplates(env.DataDir, env.EnvDir, env.Environments, env.TemplateExtension)
		var err error
		ref, err = env.BootSessions.Create(context.Background(), bootsession.Snapshot{
			Server:      server.New("06:66:de:ad:be:ef", "192.0.2.10", "boot-host"),
			Target:      "install.cfg",
			Environment: "staging",
			Params: map[string]any{
				"hostname": "boot-host",
			},
		})
		require.NoError(t, err)
	})
	req := httptest.NewRequest(http.MethodGet, "/configs/install.cfg?ref="+url.QueryEscape(ref), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "variant=staging")
	assert.Contains(t, rr.Body.String(), "baseURL=localhost:8081/env/staging")
	assert.Contains(t, rr.Body.String(), "hostname=boot-host")
}

func TestConfigTemplateRouteMissingBootReferenceReturnsNotFound(t *testing.T) {
	handler := newTestRouter(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/configs/preseed/debian?ref=missing-ref", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "boot reference not found")
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
	store := memory.New()
	env := &environment.Environment{
		BaseURL:           "localhost:8081",
		DataDir:           dataDir,
		EnvDir:            "env_overrides",
		UIDir:             filepath.Join(t.TempDir(), "missing-web"),
		TemplateExtension: ".slc",
		StaticTemplates:   staticTemplates,
		Logger:            log.MakeLogger(io.Discard),
		ServerStates:      &server.States{Servers: make(map[string]*server.State)},
		RuntimeStore:      store,
		EventLog:          event.NewLog(store, store),
		PersistenceConfig: persistence.Config{
			Retention: persistence.RetentionConfig{
				BootSessions: time.Hour,
			},
		},
	}
	env.BootSessions = bootsession.NewStore(store, store, env.PersistenceConfig.Retention.BootSessions)
	env.Templates = templates.New(env.Logger)
	if configure != nil {
		configure(env)
	}
	env.Polling = polling.NewService(env.Logger, env.ServerStates, env.MappingResolver, env.EventLog, env.Templates, env.BaseURL).WithBootSessions(env.BootSessions)
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

func boolPtr(value bool) *bool {
	return &value
}
