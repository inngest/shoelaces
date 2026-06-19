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
)

// GetBootSessionReference returns redacted metadata for one boot/config ref.
func GetBootSessionReference(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)
	ref := chi.URLParam(r, "ref")
	if ref == "" {
		http.Error(w, "boot reference is required", http.StatusBadRequest)
		return
	}
	if env.BootSessions == nil {
		http.Error(w, "boot references are not available", http.StatusBadRequest)
		return
	}

	reference, err := env.BootSessions.Inspect(r.Context(), ref)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "boot reference not found", http.StatusNotFound)
			return
		}
		env.Logger.Error("Failed to inspect boot reference", "component", "handler", "ref", ref, "err", err)
		http.Error(w, "failed to inspect boot reference", http.StatusInternalServerError)
		return
	}

	encoded, err := json.Marshal(reference)
	if err != nil {
		env.Logger.Error("Failed to marshal boot reference", "component", "handler", "ref", ref, "err", err)
		http.Error(w, "failed to marshal boot reference", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(encoded)
}
