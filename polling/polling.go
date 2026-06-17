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

package polling

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"text/template"
	"time"

	"github.com/inngest/shoelaces/event"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/server"
	"github.com/inngest/shoelaces/templates"
	"github.com/inngest/shoelaces/utils"
)

// ManualAction represent an action taken when no automatic boot is available.
type ManualAction int

const (
	startScript = "#!ipxe\n" +
		"echo Shoelaces starts polling\n" +
		"chain --autofree --replace \\\n" +
		"    http://{{.baseURL}}/poll/1/${netX/mac:hexhyp}\n" +
		"#\n" +
		"#\n" +
		"# Do\n" +
		"#    curl http://{{.baseURL}}/poll/1/06-66-de-ad-be-ef\n" +
		"# to get an idea about what the iPXE client will receive.\n"

	maxRetry = 10

	retryScript = "#!ipxe\n" +
		"prompt --key 0x02 --timeout 7000 shoelaces: Press Ctrl-B for manual override... \\\n" +
		"  && chain -ar http://{{.baseURL}}/ipxemenu \\\n" +
		"  || chain -ar http://{{.baseURL}}/poll/1/{{.macAddress}}\n\n" +
		"# Note: the iPXE client will see the above code as an endless loop.\n" +
		"# However, Shoelaces server can break that loop to enable further booting.\n"

	timeoutScript = "#!ipxe\n" +
		"echo\n" +
		"echo Shoelaces reached the maximum number of retries\n" +
		"exit\n"

	// BootAction is used when a user selects a script for the polling
	// server. The server polls once again, so it gets the selected script
	// as answer.
	BootAction ManualAction = 0
	// RetryAction is used when a server polling does not yet have a script
	// selected by the user, hence it has to retry.
	RetryAction ManualAction = 1
	// TimeoutAction is used when a server polling is timing out.
	TimeoutAction ManualAction = 2
)

// ListServers provides a list of the servers that tried to boot
// but did not match the hostname regex or network mappings.
func ListServers(serverStates *server.States) server.Servers {
	ret := make([]server.Server, 0)

	serverStates.RLock()
	for _, s := range serverStates.Servers {
		if s.Target == server.InitTarget {
			ret = append(ret, s.Server)
		}
	}
	defer serverStates.RUnlock()
	sort.Sort(server.Servers(ret))

	return ret
}

// UpdateTarget receives parameters for booting manually. When a host
// didn't match any of the automatic methods for booting, it's going to be
// put on hold. This method is called when something is finally chosen for
// that host.
func UpdateTarget(logger log.Logger, serverStates *server.States,
	resolver *mappings.Resolver, templateRenderer *templates.ShoelacesTemplates, eventLog *event.Log, baseURL string, srv server.Server,
	targetName string, _ string, params map[string]interface{}) (inputErr bool, err error) {

	if !utils.IsValidMAC(srv.Mac) {
		return true, errors.New("invalid MAC")
	}
	if resolver == nil {
		resolver, err = mappings.NewResolver(nil)
		if err != nil {
			return false, err
		}
	}

	serverStates.Lock()
	defer serverStates.Unlock()
	servers := serverStates.Servers
	if servers[srv.Mac] == nil {
		return true, errors.New("MAC is not in the booting state")
	}

	bootServer := servers[srv.Mac].Server
	result, resolvedServer, err := resolveBootTarget(resolver, mappings.ResolveRequest{
		Mac:          bootServer.Mac,
		IP:           bootServer.IP,
		Hostname:     bootServer.Hostname,
		ManualTarget: targetName,
		Params:       copyParams(params),
	}, baseURL, bootServer)
	if err != nil {
		return true, err
	}
	if !result.HasTarget() {
		return true, errors.New("manual target did not resolve to a boot script")
	}

	// Test the template before storing the selection for the polling host.
	if _, err = templateRenderer.RenderTemplate(logger, result.Target.Script, mappings.ParamsWithUsers(result.Params, result.Users), result.Target.Environment); err != nil {
		return true, err
	}

	logger.Debug("component", "polling", "msg", "Setting server override", "server", srv.Mac, "target", targetName, "script", result.Target.Script, "environment", result.Target.Environment, "hostname", resolvedServer.Hostname, "params", utils.RedactParams(result.Params))
	eventLog.AddEvent(event.UserSelection, resolvedServer, "", result.Target.Script, nil)
	servers[srv.Mac].Server = resolvedServer
	servers[srv.Mac].Target = result.Target.Script
	servers[srv.Mac].Environment = result.Target.Environment
	servers[srv.Mac].Params = result.Params
	servers[srv.Mac].Users = result.Users
	return false, nil
}

// Poll contains the main logic of Shoelaces. It uses several heuristics to find
// the right script to return, as network maps, hostname maps and manual
// selection.
func Poll(logger log.Logger, serverStates *server.States,
	resolver *mappings.Resolver,
	eventLog *event.Log, templateRenderer *templates.ShoelacesTemplates,
	baseURL string, srv server.Server) (scriptText string, err error) {

	if resolver == nil {
		resolver, err = mappings.NewResolver(nil)
		if err != nil {
			return "", err
		}
	}

	result, resolvedServer, err := resolveBootTarget(resolver, mappings.ResolveRequest{
		Mac:      srv.Mac,
		IP:       srv.IP,
		Hostname: srv.Hostname,
	}, baseURL, srv)
	if err != nil {
		return "", err
	}
	if result.HasTarget() {
		logger.Debug("component", "polling", "msg", "Host found", "where", result.MatchType, "host", srv.Hostname, "ip", srv.IP)
		eventLog.AddEvent(event.HostBoot, resolvedServer, bootTypeForMatch(result.MatchType), result.Target.Script, result.Params)
		return genBootScript(logger, templateRenderer, result.Target.Script, result.Target.Environment, result.Params, result.Users), nil
	}

	logger.Debug("component", "polling", "msg", "Host needs manual target selection", "where", result.MatchType, "mac", srv.Mac, "ip", srv.IP)
	return manualAction(logger, serverStates, templateRenderer, eventLog, baseURL, srv, targetOptions(result.AllowedTargets))
}

func manualAction(logger log.Logger, serverStates *server.States, templateRenderer *templates.ShoelacesTemplates,
	eventLog *event.Log, baseURL string, srv server.Server, allowedTargets []server.TargetOption) (scriptText string, err error) {

	script, action := chooseManualAction(logger, serverStates, eventLog, srv, allowedTargets)
	logger.Debug("component", "polling", "target-script-name", script, "action", action)

	switch action {
	case BootAction:
		setHostName(script.Params, srv.Mac)
		srv.Hostname = script.Params["hostname"].(string)
		eventLog.AddEvent(event.HostBoot, srv, event.ManualBoot, script.Name, script.Params)
		return genBootScript(logger, templateRenderer, script.Name, script.Environment, script.Params, script.Users), nil

	case RetryAction:
		return genRetryScript(logger, baseURL, srv.Mac), nil

	case TimeoutAction:
		return timeoutScript, nil

	default:
		logger.Info("component", "polling", "msg", "Unknown action")
		return "", fmt.Errorf("%s", "Unknown action")
	}
}

func chooseManualAction(logger log.Logger, serverStates *server.States,
	eventLog *event.Log, srv server.Server, allowedTargets []server.TargetOption) (*mappings.Script, ManualAction) {

	serverStates.Lock()
	defer serverStates.Unlock()

	if m := serverStates.Servers[srv.Mac]; m != nil {
		if m.Target != server.InitTarget {
			serverStates.DeleteServer(srv.Mac)
			logger.Debug("component", "polling", "msg", "Server boot", "mac", srv.Mac)
			return &mappings.Script{
				Name:        m.Target,
				Environment: m.Environment,
				Params:      m.Params,
				Users:       m.Users}, BootAction
		} else if m.Retry <= maxRetry {
			m.Retry++
			m.LastAccess = int(time.Now().UTC().Unix())
			logger.Debug("component", "polling", "msg", "Retrying reboot", "mac", srv.Mac)
			return nil, RetryAction
		} else {
			serverStates.DeleteServer(srv.Mac)
			logger.Debug("component", "polling", "msg", "Timing out server", "mac", srv.Mac)
			return nil, TimeoutAction
		}
	}

	serverStates.AddServerWithTargets(srv, allowedTargets)
	logger.Debug("component", "polling", "msg", "New server", "mac", srv.Mac)
	eventLog.AddEvent(event.HostPoll, srv, "", "", nil)

	return nil, RetryAction
}

func resolveBootTarget(resolver *mappings.Resolver, request mappings.ResolveRequest,
	baseURL string, srv server.Server) (mappings.ResolveResult, server.Server, error) {
	result, err := resolver.Resolve(request)
	if err != nil || !result.HasTarget() {
		return result, srv, err
	}

	request.GeneratedParams = generatedBootParams(result.Params, baseURL, result.Target.Environment, srv, result.MatchType)
	result, err = resolver.Resolve(request)
	if err != nil {
		return mappings.ResolveResult{}, srv, err
	}
	if hostname, ok := result.Params["hostname"].(string); ok {
		srv.Hostname = hostname
	}
	return result, srv, nil
}

func generatedBootParams(params map[string]interface{}, baseURL, envName string, srv server.Server, matchType mappings.MatchType) map[string]interface{} {
	generated := map[string]interface{}{
		"baseURL": utils.BaseURLforEnvName(baseURL, envName),
	}
	if matchType == mappings.MatchHostname && srv.Hostname != "" {
		generated["hostname"] = srv.Hostname
		return generated
	}

	hostnameParams := copyParams(params)
	setHostName(hostnameParams, srv.Mac)
	generated["hostname"] = hostnameParams["hostname"]
	return generated
}

func bootTypeForMatch(matchType mappings.MatchType) string {
	switch matchType {
	case mappings.MatchMAC:
		return "MAC Match"
	case mappings.MatchIP:
		return "IP Match"
	case mappings.MatchHostname:
		return event.PtrMatchBoot
	case mappings.MatchNetwork:
		return event.SubnetMatchBoot
	case mappings.MatchManual:
		return event.ManualBoot
	default:
		return ""
	}
}

func targetOptions(targets []mappings.TargetCandidate) []server.TargetOption {
	options := make([]server.TargetOption, 0, len(targets))
	for _, target := range targets {
		options = append(options, server.TargetOption{
			Name:        target.Name,
			Script:      target.Script,
			Label:       target.Label,
			Environment: target.Environment,
		})
	}
	return options
}

func copyParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	copied := make(map[string]interface{}, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return copied
}

func setHostName(params map[string]interface{}, mac string) {
	if _, ok := params["hostname"]; !ok {
		hostname := utils.MacColonToDash(mac)
		if hnPrefix, ok := params["hostnamePrefix"]; ok {
			hnPrefixStr, isString := hnPrefix.(string)
			if !isString {
				hnPrefixStr = ""
			}
			params["hostname"] = hnPrefixStr + hostname
		} else {
			params["hostname"] = hostname
		}
	}
}

func GenStartScript(logger log.Logger, baseURL string) string {
	variablesMap := map[string]interface{}{}
	parsedTemplate := &bytes.Buffer{}

	tmpl, err := template.New("retry").Parse(startScript)
	if err != nil {
		logger.Info("component", "polling", "msg", "Error parsing start template")
		panic(err)
	}

	variablesMap["baseURL"] = baseURL
	err = tmpl.Execute(parsedTemplate, variablesMap)
	if err != nil {
		logger.Info("component", "polling", "msg", "Error executing start template")
		panic(err)
	}

	return parsedTemplate.String()
}

func genBootScript(logger log.Logger, templateRenderer *templates.ShoelacesTemplates, scriptName, envName string, params map[string]interface{}, users map[string]mappings.ResolvedUser) string {
	text, err := templateRenderer.RenderTemplate(logger, scriptName, mappings.ParamsWithUsers(params, users), envName)
	if err != nil {
		panic(err)
	}
	return text
}

func genRetryScript(logger log.Logger, baseURL string, mac string) string {
	variablesMap := map[string]interface{}{}
	parsedTemplate := &bytes.Buffer{}

	tmpl, err := template.New("retry").Parse(retryScript)
	if err != nil {
		logger.Info("component", "polling", "msg", "Error parsing retry template", "mac", mac)
		panic(err)
	}

	variablesMap["baseURL"] = baseURL
	variablesMap["macAddress"] = utils.MacColonToDash(mac)
	err = tmpl.Execute(parsedTemplate, variablesMap)
	if err != nil {
		logger.Info("component", "polling", "msg", "Error executing retry template", "mac", mac)
		panic(err)
	}

	return parsedTemplate.String()
}
