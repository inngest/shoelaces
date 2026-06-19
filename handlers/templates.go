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
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"github.com/inngest/shoelaces/bootsession"
	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/utils"
)

// TemplateHandler handles templated config files
type TemplateHandler struct {
	env *environment.Environment
}

// TemplateHandler is the dynamic configuration provider endpoint. It
// receives a key and maybe an environment.
func (t *TemplateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	variablesMap := map[string]any{}
	configName := filepath.Clean(r.URL.Path)
	env := t.env

	if configName == "" {
		env.Logger.Error("Template request missing config name", "component", "handler", "path", r.URL.Path)
		http.Error(w, "No template name provided", http.StatusNotFound)
		return
	}

	queryParams := firstQueryValues(r)
	envName := envNameFromRequest(r)
	ref := queryParams[bootsession.QueryParam]
	if ref != "" {
		if env.BootSessions == nil {
			env.Logger.Error("Template request has ref but boot sessions are disabled", "component", "handler", "template", configName, "ref", ref)
			http.Error(w, "boot references are not available", http.StatusBadRequest)
			return
		}
		snapshot, err := env.BootSessions.Get(r.Context(), ref)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				env.Logger.Error("Template request references missing boot session", "component", "handler", "template", configName, "ref", ref)
				http.Error(w, "boot reference not found", http.StatusNotFound)
				return
			}
			env.Logger.Error("Failed to resolve boot session", "component", "handler", "template", configName, "ref", ref, "err", err)
			http.Error(w, "failed to resolve boot reference", http.StatusInternalServerError)
			return
		}
		if snapshot.Environment != "" {
			envName = snapshot.Environment
		}
		delete(queryParams, bootsession.QueryParam)

		params := make(map[string]any, len(snapshot.Params)+len(queryParams))
		for key, val := range snapshot.Params {
			params[key] = val
		}
		for key, val := range queryParams {
			params[key] = val
		}
		variablesMap = mappings.ParamsWithProvisioning(params, snapshot.Users, snapshot.Provisioning)
		queryParams = nil
	}

	for key, val := range queryParams {
		variablesMap[key] = val
	}

	variablesMap["baseURL"] = utils.BaseURLforEnvName(env.BaseURL, envName)

	configString, err := env.Templates.RenderTemplate(configName, variablesMap, envName)
	if err != nil {
		env.Logger.Error("Failed to render config template", "component", "handler", "template", configName, "environment", envName, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		_, _ = io.WriteString(w, configString)
	}
}

// TemplateHandler returns a TemplateHandler instance implementing http.Handler
func TemplateServer(env *environment.Environment) *TemplateHandler {
	return &TemplateHandler{env: env}
}

func firstQueryValues(r *http.Request) map[string]string {
	values := make(map[string]string)
	for key, val := range r.URL.Query() {
		if len(val) == 0 {
			continue
		}
		values[key] = val[0]
	}
	return values
}

// TemplateParamsHandler serves the parameters required for completing a template.
type TemplateParamsHandler struct {
	env *environment.Environment
}

// TemplateParamsServer returns a handler for template parameter discovery.
func TemplateParamsServer(env *environment.Environment) *TemplateParamsHandler {
	return &TemplateParamsHandler{env: env}
}

func (h *TemplateParamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var vars []string
	env := h.env

	filterBlacklist := func(s string) bool {
		return !utils.StringInSlice(s, env.ParamsBlacklist)
	}

	script := r.URL.Query().Get("script")
	if script == "" {
		env.Logger.Error("Template params request missing script", "component", "handler", "path", r.URL.Path)
		http.Error(w, "Required script parameter", http.StatusInternalServerError)
		return
	}

	envName := r.URL.Query().Get("environment")
	if envName == "" {
		envName = "default"
	}

	vars = utils.Filter(env.Templates.ListVariables(script, envName), filterBlacklist)

	marshaled, err := json.Marshal(vars)
	if err != nil {
		env.Logger.Error("Failed to marshal template params", "component", "handler", "script", script, "environment", envName, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(marshaled)
}
