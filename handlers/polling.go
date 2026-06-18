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
	"errors"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/utils"
)

// StartPollingHandler is called by iPXE boot agents. It returns the poll script.
func StartPollingHandler(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)

	script := env.Polling.StartScript()

	_, _ = w.Write([]byte(script))
}

// PollHandler is called by iPXE boot agents. It returns the boot script
// specified on the configuration or, if the host is unknown, it makes it
// retry for a while until the user specifies alternative IPXE boot script.
func PollHandler(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		env.Logger.Error("Failed to parse polling remote address", "component", "handler", "remote_addr", r.RemoteAddr, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	// iPXE MAC addresses come with dashes instead of colons
	mac := utils.MacDashToColon(vars["mac"])
	host := r.FormValue("host")

	err = validateMACAndIP(env.Logger, mac, ip)
	if err != nil {
		env.Logger.Error("Rejected polling request", "component", "handler", "mac", mac, "ip", ip, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if host == "" {
		host = resolveHostname(env.Logger, ip)
	}

	server := server.New(mac, ip, host)
	script, err := env.Polling.Poll(server)

	if err != nil {
		env.Logger.Error("Polling request failed", "component", "handler", "mac", mac, "ip", ip, "host", host, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(script))
}

// ServerListHandler provides a list of the servers that tried to boot
// but did not match the hostname regex or network mappings.
func ServerListHandler(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)

	servers, err := json.Marshal(env.Polling.ListServers())
	if err != nil {
		env.Logger.Error("Failed to marshal server list", "component", "handler", "err", err)
		os.Exit(1)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(servers)
}

// UpdateTargetHandler is a POST endpoint that receives parameters for
// booting manually.
func UpdateTargetHandler(w http.ResponseWriter, r *http.Request) {
	env := envFromRequest(r)

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		env.Logger.Error("Failed to parse update target remote address", "component", "handler", "remote_addr", r.RemoteAddr, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		env.Logger.Error("Failed to parse update target form", "component", "handler", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mac, scriptName, environment, params := parsePostForm(r.PostForm)
	if mac == "" || scriptName == "" {
		env.Logger.Error("Rejected update target request", "component", "handler", "mac", mac, "target", scriptName, "environment", environment, "reason", "missing required field")
		http.Error(w, "MAC address and target must not be empty", http.StatusBadRequest)
		return
	}

	server := server.New(mac, ip, "")
	inputErr, err := env.Polling.UpdateTarget(server, scriptName, environment, params)

	if err != nil {
		if inputErr {
			env.Logger.Error("Rejected update target request", "component", "handler", "mac", mac, "target", scriptName, "environment", environment, "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			env.Logger.Error("Update target request failed", "component", "handler", "mac", mac, "target", scriptName, "environment", environment, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func parsePostForm(form map[string][]string) (mac, scriptName, environment string, params map[string]interface{}) {
	params = make(map[string]interface{})
	for k, v := range form {
		switch k {
		case "mac":
			mac = utils.MacDashToColon(v[0])
		case "target":
			scriptName = v[0]
		case "environment":
			environment = v[0]
		default:
			params[k] = v[0]
		}
	}
	return
}

func validateMACAndIP(logger log.Logger, mac string, ip string) (err error) {
	if !utils.IsValidMAC(mac) {
		logger.Error("Invalid MAC", "component", "polling", "mac", mac)
		return errors.New("invalid MAC")
	}

	if !utils.IsValidIP(ip) {
		logger.Error("Invalid IP", "component", "polling", "ip", ip)
		return errors.New("invalid IP")
	}

	logger.Debug("MAC and IP validated", "component", "polling", "mac", mac, "ip", ip)

	return nil
}

func resolveHostname(logger log.Logger, ip string) string {
	host := utils.ResolveHostname(ip)
	if host == "" {
		logger.Info("Can't resolve IP", "component", "polling", "ip", ip)
	}

	return host
}
