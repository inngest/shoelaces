// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/persistence"
	"github.com/oklog/ulid/v2"
	cli "github.com/urfave/cli/v3"
)

type eventListOptions struct {
	MAC   string
	Type  *int
	Since *time.Time
	Limit int
}

type eventOutputRecord struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	MAC        string    `json:"mac"`
	IP         string    `json:"ip,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	BootType   string    `json:"boot_type,omitempty"`
	Script     string    `json:"script,omitempty"`
	Message    string    `json:"message,omitempty"`
}

func eventsCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "events",
		Usage:     "Inspect persisted event history",
		UsageText: "shoelaces [options...] events <command>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			return cli.ShowSubcommandHelp(cmd)
		},
		Commands: []*cli.Command{
			eventsListCommand(configValues),
			eventsGetCommand(configValues),
		},
	}
}

func eventsListCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List persisted events",
		UsageText: "shoelaces [options...] events list [options...]",
		Flags: []cli.Flag{
			inspectionOutputFlag(),
			&cli.StringFlag{
				Name:  "mac",
				Usage: "Only show events for a MAC address",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Only show events with a type: host-poll, user-selection, host-boot, or host-timeout",
			},
			&cli.StringFlag{
				Name:  "since",
				Usage: "Only show events at or after an RFC3339 timestamp",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of events to show",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			format, err := outputFormatFromCommand(cmd)
			if err != nil {
				return err
			}
			options, err := inspectionOptionsFromConfig(configValues)
			if err != nil {
				return err
			}
			listOptions, err := eventListOptionsFromCommand(cmd)
			if err != nil {
				return err
			}

			return withInspectionStore(ctx, options, func(store *inspectionStore) error {
				events, err := store.Queries.ListEvents(ctx)
				if err != nil {
					return err
				}
				records := eventOutputRecords(filterEventRecords(events, listOptions))
				return writeEventOutput(cmd.Writer, format, records)
			})
		},
	}
}

func eventsGetCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get one persisted event",
		UsageText: "shoelaces [options...] events get [options...] <event-id>",
		Flags: []cli.Flag{
			inspectionOutputFlag(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("expected exactly one event id")
			}
			id, err := ulid.Parse(cmd.Args().First())
			if err != nil {
				return fmt.Errorf("invalid event id %q: %w", cmd.Args().First(), err)
			}
			format, err := outputFormatFromCommand(cmd)
			if err != nil {
				return err
			}
			options, err := inspectionOptionsFromConfig(configValues)
			if err != nil {
				return err
			}

			return withInspectionStore(ctx, options, func(store *inspectionStore) error {
				record, err := store.Queries.GetEvent(ctx, id)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return fmt.Errorf("event not found: %s", id.String())
					}
					return err
				}
				return writeEventRecordOutput(cmd.Writer, format, eventOutputRecordFromRecord(record))
			})
		},
	}
}

func eventListOptionsFromCommand(cmd *cli.Command) (eventListOptions, error) {
	options := eventListOptions{
		MAC:   strings.ToLower(cmd.String("mac")),
		Limit: cmd.Int("limit"),
	}
	if options.Limit < 0 {
		return eventListOptions{}, fmt.Errorf("limit must be greater than or equal to 0")
	}

	rawType := cmd.String("type")
	if rawType != "" {
		eventType, err := parseEventType(rawType)
		if err != nil {
			return eventListOptions{}, err
		}
		options.Type = &eventType
	}

	rawSince := cmd.String("since")
	if rawSince != "" {
		since, err := time.Parse(time.RFC3339, rawSince)
		if err != nil {
			return eventListOptions{}, fmt.Errorf("invalid since timestamp %q: use RFC3339", rawSince)
		}
		options.Since = &since
	}

	return options, nil
}

func parseEventType(raw string) (int, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "host-poll", "host_poll", "poll":
		return int(event.HostPoll), nil
	case "user-selection", "user_selection", "selection":
		return int(event.UserSelection), nil
	case "host-boot", "host_boot", "boot":
		return int(event.HostBoot), nil
	case "host-timeout", "host_timeout", "timeout":
		return int(event.HostTimeout), nil
	}
	eventType, err := strconv.Atoi(normalized)
	if err == nil {
		switch event.Type(eventType) {
		case event.HostPoll, event.UserSelection, event.HostBoot, event.HostTimeout:
			return eventType, nil
		}
	}
	return 0, fmt.Errorf("unsupported event type %q; use host-poll, user-selection, host-boot, or host-timeout", raw)
}

func eventTypeName(eventType int) string {
	switch event.Type(eventType) {
	case event.HostPoll:
		return "host-poll"
	case event.UserSelection:
		return "user-selection"
	case event.HostBoot:
		return "host-boot"
	case event.HostTimeout:
		return "host-timeout"
	default:
		return strconv.Itoa(eventType)
	}
}

func filterEventRecords(records []persistence.EventRecord, options eventListOptions) []persistence.EventRecord {
	filtered := make([]persistence.EventRecord, 0, len(records))
	for _, record := range records {
		if options.MAC != "" && strings.ToLower(record.MAC) != options.MAC {
			continue
		}
		if options.Type != nil && record.Type != *options.Type {
			continue
		}
		if options.Since != nil && record.OccurredAt.Before(*options.Since) {
			continue
		}
		filtered = append(filtered, record)
		if options.Limit > 0 && len(filtered) >= options.Limit {
			break
		}
	}
	return filtered
}

func eventOutputRecords(records []persistence.EventRecord) []eventOutputRecord {
	output := make([]eventOutputRecord, len(records))
	for i, record := range records {
		output[i] = eventOutputRecordFromRecord(record)
	}
	return output
}

func eventOutputRecordFromRecord(record persistence.EventRecord) eventOutputRecord {
	return eventOutputRecord{
		ID:         record.ID.String(),
		Type:       eventTypeName(record.Type),
		OccurredAt: record.OccurredAt,
		MAC:        record.MAC,
		IP:         record.IP,
		Hostname:   record.Hostname,
		BootType:   record.BootType,
		Script:     record.Script,
		Message:    record.Message,
	}
}

func writeEventOutput(w interface {
	Write([]byte) (int, error)
}, format outputFormat, records []eventOutputRecord) error {
	switch format {
	case outputFormatJSON:
		return writeJSONOutput(w, records)
	case outputFormatTable:
		return writeTableOutput(w, eventTableOutput(records))
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}

func writeEventRecordOutput(w interface {
	Write([]byte) (int, error)
}, format outputFormat, record eventOutputRecord) error {
	switch format {
	case outputFormatJSON:
		return writeJSONOutput(w, record)
	case outputFormatTable:
		return writeTableOutput(w, eventTableOutput([]eventOutputRecord{record}))
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}

func eventTableOutput(records []eventOutputRecord) tableOutput {
	table := tableOutput{
		Headers: []string{"TIME", "EVENT ID", "MAC", "HOSTNAME", "TYPE", "SCRIPT", "MESSAGE"},
		Rows:    make([][]string, len(records)),
	}
	for i, record := range records {
		table.Rows[i] = []string{
			record.OccurredAt.Format(time.RFC3339),
			record.ID,
			record.MAC,
			record.Hostname,
			record.Type,
			record.Script,
			record.Message,
		}
	}
	return table
}
