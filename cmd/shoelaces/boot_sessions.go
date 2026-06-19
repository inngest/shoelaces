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
	"time"

	"github.com/inngest/shoelaces/bootsession"
	cli "github.com/urfave/cli/v3"
)

func bootSessionsCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "boot-sessions",
		Usage:     "Inspect persisted boot-session references",
		UsageText: "shoelaces [options...] boot-sessions <command>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			return cli.ShowSubcommandHelp(cmd)
		},
		Commands: []*cli.Command{
			bootSessionsGetCommand(configValues),
		},
	}
}

func bootSessionsGetCommand(configValues map[any]any) *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "Get one persisted boot-session reference",
		UsageText: "shoelaces [options...] boot-sessions get [options...] <ref>",
		Flags: []cli.Flag{
			inspectionOutputFlag(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return fmt.Errorf("expected exactly one boot-session ref")
			}
			ref := cmd.Args().First()
			format, err := outputFormatFromCommand(cmd)
			if err != nil {
				return err
			}
			options, err := inspectionOptionsFromConfig(configValues)
			if err != nil {
				return err
			}

			return withInspectionStore(ctx, options, func(store *inspectionStore) error {
				bootSessions := bootsession.NewStore(nil, store.Queries, 0)
				reference, err := bootSessions.Inspect(ctx, ref)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return fmt.Errorf("boot session not found: %s", ref)
					}
					return err
				}
				return writeBootSessionReferenceOutput(cmd.Writer, format, reference)
			})
		},
	}
}

func writeBootSessionReferenceOutput(w interface {
	Write([]byte) (int, error)
}, format outputFormat, reference bootsession.Reference) error {
	switch format {
	case outputFormatJSON:
		return writeJSONOutput(w, reference)
	case outputFormatTable:
		return writeTableOutput(w, bootSessionReferenceTableOutput(reference))
	default:
		return fmt.Errorf("unsupported output format %q; use table or json", format)
	}
}

func bootSessionReferenceTableOutput(reference bootsession.Reference) tableOutput {
	return tableOutput{
		Headers: []string{"REF", "MAC", "HOSTNAME", "TARGET", "ENVIRONMENT", "CREATED", "EXPIRES"},
		Rows: [][]string{
			{
				reference.Ref,
				reference.Server.Mac,
				reference.Server.Hostname,
				reference.Target,
				reference.Environment,
				reference.CreatedAt.Format(time.RFC3339),
				reference.ExpiresAt.Format(time.RFC3339),
			},
		},
	}
}
