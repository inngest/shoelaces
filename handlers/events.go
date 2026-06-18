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
	"encoding/json"
	"net/http"
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
