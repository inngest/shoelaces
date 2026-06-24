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

package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/handlers"
)

// ShoelacesRouter sets up all routes and handlers for shoelaces
func ShoelacesRouter(env *environment.Environment) http.Handler {
	r := chi.NewRouter()

	// Main UI page
	r.Method(http.MethodGet, "/", handlers.RenderDefaultTemplate("index", env))
	// Event Log History page
	r.Method(http.MethodGet, "/events", handlers.RenderDefaultTemplate("events", env))
	// Currently configured mappings page
	r.Method(http.MethodGet, "/mappings", handlers.RenderDefaultTemplate("mappings", env))
	// Static files used by the UI
	r.Handle("/static/*", http.StripPrefix("/static/", staticFileServer(env)))
	// Manual boot parameters POST endpoint
	r.Post("/update/target", handlers.UpdateTargetHandler)
	// Provides a list of the servers that tried to boot but did not match
	// the hostname regex or network mappings
	r.Get("/ajax/servers", handlers.ServerListHandler)
	// Event Log History JSON endpoint
	r.Get("/ajax/events", handlers.ListEvents)
	// Event Log single-record JSON endpoint
	r.Get("/ajax/events/{id}", handlers.GetEvent)
	// Redacted boot/config reference JSON endpoint
	r.Get("/ajax/boot-sessions/{ref}", handlers.GetBootSessionReference)
	// Provides the list of possible parameters for a given template
	r.Handle("/ajax/script/params", handlers.TemplateParamsServer(env))

	// Static configuration files endpoint
	r.Handle("/configs/static/*", http.StripPrefix("/configs/static/", handlers.StaticConfigFileServer()))

	// Ref-scoped generated helper endpoint
	r.Handle("/configs/generated/*", http.StripPrefix("/configs/generated/", handlers.GeneratedConfigServer(env)))

	// Dynamic configuration endpoint
	r.Handle("/configs/*", http.StripPrefix("/configs/", handlers.TemplateServer(env)))

	// Starting point for iPXE boot agents, usualy defined by DHCP server.
	// Gets the iPXE boot agents into the polling loop.
	r.Get("/start", handlers.StartPollingHandler)
	// Called by iPXE boot agents, returns boot script specified on the configuration
	// or if the host is unknown makes it retry for a while until the user specifies
	// alternative ipxe boot script
	r.Get("/poll/1/{mac}", handlers.PollHandler)
	// Serves a generated iPXE boot script providing a selection
	// of all of the boot scripts available on the filesystem for that environment.
	r.Get("/ipxemenu", handlers.IPXEMenu)

	return r
}

func staticFileServer(env *environment.Environment) http.Handler {
	if env.UsesUIOverride() {
		return http.FileServer(http.Dir(env.UIDir))
	}

	return http.FileServer(http.FS(shoelaces.StaticFS()))
}
