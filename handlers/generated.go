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
	"io"
	"net/http"
	"path/filepath"

	"github.com/inngest/shoelaces/environment"
)

// GeneratedConfigHandler serves helper artifacts that must be rendered from a
// boot reference, such as installer scripts that need resolved host secrets.
type GeneratedConfigHandler struct {
	env *environment.Environment
}

func GeneratedConfigServer(env *environment.Environment) *GeneratedConfigHandler {
	return &GeneratedConfigHandler{env: env}
}

func (h *GeneratedConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	configName := filepath.ToSlash(filepath.Clean(r.URL.Path))
	if configName == "" || configName == "." {
		h.env.Logger.Error("Generated template request missing config name", "component", "handler", "path", r.URL.Path)
		http.Error(w, "No generated config name provided", http.StatusNotFound)
		return
	}
	templateName := "generated/" + configName

	resolved, ok := resolveTemplateRequest(w, r, h.env, templateName, true)
	if !ok {
		return
	}

	configString, err := h.env.Templates.RenderTemplate(templateName, resolved.variables, resolved.envName)
	if err != nil {
		h.env.Logger.Error("Failed to render generated config template", "component", "handler", "template", templateName, "environment", resolved.envName, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = io.WriteString(w, configString)
}
