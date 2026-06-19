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
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/inngest/shoelaces/persistence"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/utils"
	cli "github.com/urfave/cli/v3"
)

type serverListOptions struct {
	MAC     string
	Waiting bool
}

type serverStateOutputRecord struct {
	MAC            string                     `json:"mac"`
	IP             string                     `json:"ip,omitempty"`
	Hostname       string                     `json:"hostname,omitempty"`
	Target         string                     `json:"target"`
	Environment    string                     `json:"environment,omitempty"`
	Waiting        bool                       `json:"waiting"`
	Retry          int64                      `json:"retry"`
	LastAccess     time.Time                  `json:"last_access"`
	AllowedTargets []serverTargetOutputRecord `json:"allowed_targets,omitempty"`
	Params         map[string]any             `json:"params,omitempty"`
}

type serverTargetOutputRecord struct {
	Name        string `json:"name"`
	Script      string `json:"script,omitempty"`
	Label       string `json:"label,omitempty"`
	Environment string `json:"environment,omitempty"`
}

func serversCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "servers",
		Usage:     "Inspect persisted server state",
		UsageText: "shoelaces [options...] servers <command>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			return cli.ShowSubcommandHelp(cmd)
		},
		Commands: []*cli.Command{
			serversListCommand(configValues),
			serversGetCommand(configValues),
		},
	}
}

func serversListCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List persisted server state",
		UsageText: "shoelaces [options...] servers list [options...]",
		Flags: []cli.Flag{
			inspectionOutputFlag(),
			&cli.StringFlag{
				Name:  "mac",
				Usage: "Only show server state for a MAC address",
			},
			&cli.BoolFlag{
				Name:  "waiting",
				Usage: "Only show hosts waiting for manual selection",
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
			listOptions := serverListOptionsFromCommand(cmd)

			return withInspectionStore(ctx, options, func(store *inspectionStore) error {
				states, err := store.Queries.ListServerStates(ctx)
				if err != nil {
					return err
				}
				records, err := serverStateOutputRecords(filterServerStateRecords(states, listOptions))
				if err != nil {
					return err
				}
				return writeServerStateOutput(cmd.Writer, format, records)
			})
		},
	}
}

func serversGetCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get persisted server state for one MAC",
		UsageText: "shoelaces [options...] servers get [options...] <mac>",
		Flags: []cli.Flag{
			inspectionOutputFlag(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("expected exactly one MAC address")
			}
			mac := strings.ToLower(cmd.Args().First())
			format, err := outputFormatFromCommand(cmd)
			if err != nil {
				return err
			}
			options, err := inspectionOptionsFromConfig(configValues)
			if err != nil {
				return err
			}

			return withInspectionStore(ctx, options, func(store *inspectionStore) error {
				state, err := store.Queries.GetServerState(ctx, mac)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return fmt.Errorf("server state not found: %s", mac)
					}
					return err
				}
				record, err := serverStateOutputRecordFromRecord(state)
				if err != nil {
					return err
				}
				return writeServerStateRecordOutput(cmd.Writer, format, record)
			})
		},
	}
}

func serverListOptionsFromCommand(cmd *cli.Command) serverListOptions {
	return serverListOptions{
		MAC:     strings.ToLower(cmd.String("mac")),
		Waiting: cmd.Bool("waiting"),
	}
}

func filterServerStateRecords(records []persistence.ServerStateRecord, options serverListOptions) []persistence.ServerStateRecord {
	filtered := make([]persistence.ServerStateRecord, 0, len(records))
	for _, record := range records {
		if options.MAC != "" && strings.ToLower(record.MAC) != options.MAC {
			continue
		}
		if options.Waiting && !serverStateIsWaiting(record) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func serverStateOutputRecords(records []persistence.ServerStateRecord) ([]serverStateOutputRecord, error) {
	output := make([]serverStateOutputRecord, len(records))
	for i, record := range records {
		converted, err := serverStateOutputRecordFromRecord(record)
		if err != nil {
			return nil, err
		}
		output[i] = converted
	}
	return output, nil
}

func serverStateOutputRecordFromRecord(record persistence.ServerStateRecord) (serverStateOutputRecord, error) {
	params, err := decodeObjectJSON(record.ParamsJSON, "params")
	if err != nil {
		return serverStateOutputRecord{}, err
	}
	allowedTargets, err := decodeAllowedTargets(record.AllowedTargetsJSON)
	if err != nil {
		return serverStateOutputRecord{}, err
	}

	return serverStateOutputRecord{
		MAC:            record.MAC,
		IP:             record.IP,
		Hostname:       record.Hostname,
		Target:         record.Target,
		Environment:    record.Environment,
		Waiting:        serverStateIsWaiting(record),
		Retry:          record.Retry,
		LastAccess:     record.LastAccess,
		AllowedTargets: serverTargetOutputRecords(allowedTargets),
		Params:         utils.RedactParams(params),
	}, nil
}

func serverStateIsWaiting(record persistence.ServerStateRecord) bool {
	return record.Target == "" || record.Target == server.InitTarget
}

func decodeObjectJSON(encoded []byte, field string) (map[string]any, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", field, err)
	}
	if len(decoded) == 0 {
		return nil, nil
	}
	return decoded, nil
}

func decodeAllowedTargets(encoded []byte) ([]server.TargetOption, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	var targets []server.TargetOption
	if err := json.Unmarshal(encoded, &targets); err != nil {
		return nil, fmt.Errorf("decode allowed targets: %w", err)
	}
	if len(targets) == 0 {
		return nil, nil
	}
	return targets, nil
}

func serverTargetOutputRecords(targets []server.TargetOption) []serverTargetOutputRecord {
	if len(targets) == 0 {
		return nil
	}
	output := make([]serverTargetOutputRecord, len(targets))
	for i, target := range targets {
		output[i] = serverTargetOutputRecord{
			Name:        target.Name,
			Script:      target.Script,
			Label:       target.Label,
			Environment: target.Environment,
		}
	}
	return output
}

func writeServerStateOutput(w interface {
	Write([]byte) (int, error)
}, format outputFormat, records []serverStateOutputRecord) error {
	switch format {
	case outputFormatJSON:
		return writeJSONOutput(w, records)
	case outputFormatTable:
		return writeTableOutput(w, serverStateTableOutput(records))
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}

func writeServerStateRecordOutput(w interface {
	Write([]byte) (int, error)
}, format outputFormat, record serverStateOutputRecord) error {
	switch format {
	case outputFormatJSON:
		return writeJSONOutput(w, record)
	case outputFormatTable:
		return writeTableOutput(w, serverStateTableOutput([]serverStateOutputRecord{record}))
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}

func serverStateTableOutput(records []serverStateOutputRecord) tableOutput {
	table := tableOutput{
		Headers: []string{"MAC", "IP", "HOSTNAME", "TARGET", "ENVIRONMENT", "RETRY", "LAST ACCESS", "ALLOWED TARGETS", "PARAMS"},
		Rows:    make([][]string, len(records)),
	}
	for i, record := range records {
		table.Rows[i] = []string{
			record.MAC,
			record.IP,
			record.Hostname,
			record.Target,
			record.Environment,
			fmt.Sprintf("%d", record.Retry),
			record.LastAccess.Format(time.RFC3339),
			allowedTargetNames(record.AllowedTargets),
			paramsSummary(record.Params),
		}
	}
	return table
}

func allowedTargetNames(targets []serverTargetOutputRecord) string {
	if len(targets) == 0 {
		return ""
	}
	names := make([]string, len(targets))
	for i, target := range targets {
		names[i] = target.Name
	}
	return strings.Join(names, ",")
}

func paramsSummary(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
