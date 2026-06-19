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
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
)

// ListEvents returns a JSON list of the logged events.
func ListEvents(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)
	events, err := env.EventLog.ListEvents(r.Context())
	if err != nil {
		env.Logger.Error("Failed to list events", "component", "handler", "err", err)
		http.Error(w, "failed to list events", http.StatusInternalServerError)
		return
	}
	eventList, err := json.Marshal(events)
	if err != nil {
		env.Logger.Error("Failed to marshal events", "component", "handler", "err", err)
		http.Error(w, "failed to marshal events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(eventList)
}

// GetEvent returns one logged event by public event ID.
func GetEvent(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)
	id := chi.URLParam(r, "id")
	if _, err := ulid.Parse(id); err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	event, err := env.EventLog.GetEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		env.Logger.Error("Failed to get event", "component", "handler", "event_id", id, "err", err)
		http.Error(w, "failed to get event", http.StatusInternalServerError)
		return
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		env.Logger.Error("Failed to marshal event", "component", "handler", "event_id", id, "err", err)
		http.Error(w, "failed to marshal event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(encoded)
}
