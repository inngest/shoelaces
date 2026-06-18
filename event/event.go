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

package event

import (
	"context"
	"encoding/json"
	"time"

	"github.com/inngest/shoelaces/persistence"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/utils"
	"github.com/oklog/ulid/v2"
)

// Type holds the different typs of events
type Type int

const (
	// HostPoll is the event generated when a host poll Shoelaces for a script
	HostPoll Type = 0
	// UserSelection is the event generated when a user selects a script and hits "Boot!"
	UserSelection Type = 1
	// HostBoot is the event generated when a host finally boots
	HostBoot Type = 2
	// HostTimeout is the event generated when a host polls and after some
	// minutes without activity, timeouts.
	HostTimeout Type = 3

	// PtrMatchBoot is triggered when a PTR is matched to an IP
	PtrMatchBoot = "DNS Match"
	// SubnetMatchBoot is triggered when an IP matches a subnet mapping
	SubnetMatchBoot = "Subnet Match"
	// ManualBoot is triggered when the user selects manual boot
	ManualBoot = "Manual"
)

// Event holds information related to the interactions of hosts when they boot.
// It's used exclusively in the Shoelaces web frontend.
type Event struct {
	// ID is a stable, sortable event identifier exposed by the events API.
	ID ulid.ULID `json:"id"`
	// Type classifies the event for the UI.
	Type Type `json:"eventType"`
	// OccurredAt records when Shoelaces observed or accepted the event.
	OccurredAt time.Time `json:"occurred_at"`
	// Server is the host that polled, booted, or received a manual selection.
	Server server.Server `json:"server"`
	// BootType describes how the boot target was selected.
	BootType string `json:"bootType"`
	// Script is the selected boot script name, when one exists.
	Script string `json:"script"`
	// Message is the pre-rendered operator-facing event summary.
	Message string `json:"message"`
	// Params contains already-redacted template parameters captured with the event.
	Params map[string]any `json:"params"`
}

// Log records and queries Shoelaces UI/audit events.
//
// Events is kept as a compatibility mirror for legacy callers and tests, but
// production reads should use ListEvents so durable backends can be queried.
type Log struct {
	// Events mirrors appended events for legacy in-memory callers.
	Events map[string][]Event
	// commands writes already-redacted event records to the persistence backend.
	commands persistence.EventCommands
	// queries reads persisted event records for UI/API responses.
	queries persistence.EventQueries
	// now provides deterministic timestamps in tests.
	now func() time.Time
	// newID provides deterministic event IDs in tests.
	newID func() ulid.ULID
}

// New creates a new Event object
func New(eventType Type, srv server.Server, bootType, script string, params map[string]any) Event {
	return newWithID(ulid.Make(), eventType, srv, bootType, script, params, time.Now())
}

func newWithID(id ulid.ULID, eventType Type, srv server.Server, bootType, script string, params map[string]any, occurredAt time.Time) Event {
	var event Event

	event.ID = id
	event.Type = eventType
	event.OccurredAt = occurredAt
	event.Server = srv
	event.BootType = bootType
	event.Script = script
	event.Params = utils.RedactParams(params)

	event.setMessage()

	return event
}

// NewLog returns an event log backed by explicit write and read interfaces.
func NewLog(commands persistence.EventCommands, queries persistence.EventQueries) *Log {
	return &Log{
		Events:   make(map[string][]Event),
		commands: commands,
		queries:  queries,
		now:      time.Now,
		newID:    ulid.Make,
	}
}

func (e *Event) setMessage() {
	switch e.Type {
	case HostPoll:
		e.Message = "Host " + e.Server.Hostname + " polled for a script."
	case UserSelection:
		e.Message = "A user selected " + e.Script + " for the host " + e.Server.Hostname + "."
	case HostBoot:
		params, _ := json.Marshal(e.Params)
		e.Message = "Host " + e.Server.Hostname + " booted using " + e.BootType + " method with the following parameters: " + string(params)
	case HostTimeout:
		e.Message = "Host " + e.Server.Hostname + " timed out."
	}
}

// AddEvent adds an Event into the event log
func (el *Log) AddEvent(eventType Type, srv server.Server, bootType string, script string, params map[string]any) {
	_ = el.AppendEvent(context.Background(), eventType, srv, bootType, script, params)
}

// AppendEvent adds a redacted event through the write-side event interface.
func (el *Log) AppendEvent(ctx context.Context, eventType Type, srv server.Server, bootType string, script string, params map[string]any) error {
	if el.now == nil {
		el.now = time.Now
	}
	if el.newID == nil {
		el.newID = ulid.Make
	}
	event := newWithID(el.newID(), eventType, srv, bootType, script, params, el.now())
	if el.Events == nil {
		el.Events = make(map[string][]Event)
	}
	el.Events[srv.Mac] = append(el.Events[srv.Mac], event)

	if el.commands == nil {
		return nil
	}
	_, err := el.commands.AppendEvent(ctx, eventToRecord(event))
	return err
}

// ListEvents returns events grouped by MAC in the JSON shape expected by the UI.
func (el *Log) ListEvents(ctx context.Context) (map[string][]Event, error) {
	if el.queries == nil {
		return copyGroupedEvents(el.Events), nil
	}

	records, err := el.queries.ListEvents(ctx)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]Event)
	for _, record := range records {
		event, err := recordToEvent(record)
		if err != nil {
			return nil, err
		}
		grouped[event.Server.Mac] = append(grouped[event.Server.Mac], event)
	}
	return grouped, nil
}

// DeleteEventsBefore removes persisted events older than the supplied cutoff.
func (el *Log) DeleteEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if el.commands == nil {
		return 0, nil
	}
	return el.commands.DeleteEventsBefore(ctx, cutoff)
}

func eventToRecord(event Event) persistence.EventRecord {
	params, _ := json.Marshal(event.Params)
	return persistence.EventRecord{
		ID:         event.ID,
		Type:       int(event.Type),
		OccurredAt: event.OccurredAt,
		MAC:        event.Server.Mac,
		IP:         event.Server.IP,
		Hostname:   event.Server.Hostname,
		BootType:   event.BootType,
		Script:     event.Script,
		Message:    event.Message,
		ParamsJSON: params,
	}
}

func recordToEvent(record persistence.EventRecord) (Event, error) {
	event := Event{
		ID:         record.ID,
		Type:       Type(record.Type),
		OccurredAt: record.OccurredAt,
		Server:     server.New(record.MAC, record.IP, record.Hostname),
		BootType:   record.BootType,
		Script:     record.Script,
		Message:    record.Message,
	}
	if len(record.ParamsJSON) > 0 {
		if err := json.Unmarshal(record.ParamsJSON, &event.Params); err != nil {
			return Event{}, err
		}
	}
	return event, nil
}

func copyGroupedEvents(events map[string][]Event) map[string][]Event {
	copied := make(map[string][]Event, len(events))
	for mac, hostEvents := range events {
		copied[mac] = append([]Event(nil), hostEvents...)
	}
	return copied
}
