// Copyright 2018 ThousandEyes Inc.
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
	"fmt"
	"net/http"
	"os"

	"github.com/inngest/shoelaces/environment"
	"github.com/inngest/shoelaces/handlers"
	"github.com/inngest/shoelaces/router"
	"github.com/inngest/shoelaces/tftpserver"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cmd, err := newCommand(os.Args, runServer)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(env *environment.Environment) error {
	app := handlers.MiddlewareChain(env).Then(router.ShoelacesRouter(env))

	if env.TFTP != nil && env.TFTP.Enabled {
		tf := tftpserver.New(env.TFTP.Addr, env.TFTP.Root, env.TFTP.Readonly, env.TFTP.Timeout).WithLogger(env.Logger)
		go func() {
			if err := tf.ListenAndServe(); err != nil {
				env.Logger.Error("TFTP server failed", "component", "tftp", "err", err)
			}
		}()
	}

	env.Logger.Info("listening", "component", "main", "transport", "http", "addr", env.BindAddr)
	return http.ListenAndServe(env.BindAddr, app)
}

func versionString() string {
	return fmt.Sprintf("shoelaces %s\ncommit: %s\ndate: %s\nbuilt by: %s\n", version, commit, date, builtBy)
}
