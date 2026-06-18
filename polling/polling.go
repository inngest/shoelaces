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
	"context"
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

// Service owns the dependencies needed for polling and manual target updates.
// It keeps handlers from threading the same logger and runtime state through
// every polling call.
type Service struct {
	logger           log.Logger
	serverStates     *server.States
	resolver         *mappings.Resolver
	eventLog         *event.Log
	templateRenderer *templates.ShoelacesTemplates
	baseURL          string
}

func NewService(logger log.Logger, serverStates *server.States, resolver *mappings.Resolver, eventLog *event.Log, templateRenderer *templates.ShoelacesTemplates, baseURL string) *Service {
	return &Service{
		logger:           logger,
		serverStates:     serverStates,
		resolver:         resolver,
		eventLog:         eventLog,
		templateRenderer: templateRenderer,
		baseURL:          baseURL,
	}
}

// ListServers returns the hosts currently waiting for manual target selection.
func (s *Service) ListServers() server.Servers {
	return ListServers(s.serverStates)
}

// StartScript renders the initial iPXE polling script.
func (s *Service) StartScript() string {
	return genStartScript(s.logger, s.baseURL)
}

// UpdateTarget stores a manually selected target for a polling host.
func (s *Service) UpdateTarget(srv server.Server, targetName string, environment string, params map[string]any) (inputErr bool, err error) {
	if !utils.IsValidMAC(srv.Mac) {
		return true, errors.New("invalid MAC")
	}
	if s.resolver == nil {
		s.resolver, err = mappings.NewResolver(nil)
		if err != nil {
			return false, err
		}
		s.resolver.WithLogger(s.logger)
	}

	s.serverStates.Lock()
	defer s.serverStates.Unlock()
	servers := s.serverStates.Servers
	if servers[srv.Mac] == nil {
		return true, errors.New("MAC is not in the booting state")
	}

	bootServer := servers[srv.Mac].Server
	result, resolvedServer, err := resolveBootTarget(s.resolver, mappings.ResolveRequest{
		Mac:          bootServer.Mac,
		IP:           bootServer.IP,
		Hostname:     bootServer.Hostname,
		ManualTarget: targetName,
		Params:       copyParams(params),
	}, s.baseURL, bootServer)
	if err != nil {
		return true, err
	}
	if !result.HasTarget() {
		return true, errors.New("manual target did not resolve to a boot script")
	}

	// Test the template before storing the selection for the polling host.
	if _, err = s.templateRenderer.RenderTemplate(result.Target.Script, mappings.ParamsWithProvisioning(result.Params, result.Users, result.Provisioning), result.Target.Environment); err != nil {
		return true, err
	}

	s.logger.Debug("Setting server override", "component", "polling", "server", srv.Mac, "target", targetName, "script", result.Target.Script, "environment", result.Target.Environment, "hostname", resolvedServer.Hostname, "params", utils.RedactParams(result.Params))
	if err := s.eventLog.AppendEvent(context.Background(), event.UserSelection, resolvedServer, "", result.Target.Script, nil); err != nil {
		return false, err
	}
	servers[srv.Mac].Server = resolvedServer
	servers[srv.Mac].Target = result.Target.Script
	servers[srv.Mac].Environment = result.Target.Environment
	servers[srv.Mac].Params = result.Params
	servers[srv.Mac].Users = result.Users
	servers[srv.Mac].Provisioning = result.Provisioning
	return false, nil
}

// Poll resolves and renders the next script for a booting host.
func (s *Service) Poll(srv server.Server) (scriptText string, err error) {
	if s.resolver == nil {
		s.resolver, err = mappings.NewResolver(nil)
		if err != nil {
			return "", err
		}
		s.resolver.WithLogger(s.logger)
	}

	result, resolvedServer, err := resolveBootTarget(s.resolver, mappings.ResolveRequest{
		Mac:      srv.Mac,
		IP:       srv.IP,
		Hostname: srv.Hostname,
	}, s.baseURL, srv)
	if err != nil {
		return "", err
	}
	if result.HasTarget() {
		s.logger.Debug("Host found", "component", "polling", "where", result.MatchType, "host", srv.Hostname, "ip", srv.IP)
		if err := s.eventLog.AppendEvent(context.Background(), event.HostBoot, resolvedServer, bootTypeForMatch(result.MatchType), result.Target.Script, result.Params); err != nil {
			return "", err
		}
		return s.genBootScript(result.Target.Script, result.Target.Environment, result.Params, result.Users, result.Provisioning), nil
	}

	s.logger.Debug("Host needs manual target selection", "component", "polling", "where", result.MatchType, "mac", srv.Mac, "ip", srv.IP)
	return s.manualAction(srv, targetOptions(result.AllowedTargets))
}

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
	targetName string, _ string, params map[string]any) (inputErr bool, err error) {
	return NewService(logger, serverStates, resolver, eventLog, templateRenderer, baseURL).UpdateTarget(srv, targetName, "", params)
}

// Poll contains the main logic of Shoelaces. It uses several heuristics to find
// the right script to return, as network maps, hostname maps and manual
// selection.
func Poll(logger log.Logger, serverStates *server.States,
	resolver *mappings.Resolver,
	eventLog *event.Log, templateRenderer *templates.ShoelacesTemplates,
	baseURL string, srv server.Server) (scriptText string, err error) {
	return NewService(logger, serverStates, resolver, eventLog, templateRenderer, baseURL).Poll(srv)
}

func (s *Service) manualAction(srv server.Server, allowedTargets []server.TargetOption) (scriptText string, err error) {

	script, action := s.chooseManualAction(srv, allowedTargets)
	s.logger.Debug("Manual action selected", "component", "polling", "target-script-name", script, "action", action)

	switch action {
	case BootAction:
		setHostName(script.Params, srv.Mac)
		srv.Hostname = script.Params["hostname"].(string)
		if err := s.eventLog.AppendEvent(context.Background(), event.HostBoot, srv, event.ManualBoot, script.Name, script.Params); err != nil {
			return "", err
		}
		return s.genBootScript(script.Name, script.Environment, script.Params, script.Users, script.Provisioning), nil

	case RetryAction:
		return s.genRetryScript(srv.Mac), nil

	case TimeoutAction:
		return timeoutScript, nil

	default:
		s.logger.Info("Unknown action", "component", "polling")
		return "", fmt.Errorf("%s", "Unknown action")
	}
}

func (s *Service) chooseManualAction(srv server.Server, allowedTargets []server.TargetOption) (*mappings.Script, ManualAction) {

	s.serverStates.Lock()
	defer s.serverStates.Unlock()

	if m := s.serverStates.Servers[srv.Mac]; m != nil {
		if m.Target != server.InitTarget {
			s.serverStates.DeleteServer(srv.Mac)
			s.logger.Debug("Server boot", "component", "polling", "mac", srv.Mac)
			return &mappings.Script{
				Name:         m.Target,
				Environment:  m.Environment,
				Params:       m.Params,
				Users:        m.Users,
				Provisioning: m.Provisioning}, BootAction
		} else if m.Retry <= maxRetry {
			m.Retry++
			m.LastAccess = int(time.Now().UTC().Unix())
			s.logger.Debug("Retrying reboot", "component", "polling", "mac", srv.Mac)
			return nil, RetryAction
		} else {
			s.serverStates.DeleteServer(srv.Mac)
			s.logger.Debug("Timing out server", "component", "polling", "mac", srv.Mac)
			return nil, TimeoutAction
		}
	}

	s.serverStates.AddServerWithTargets(srv, allowedTargets)
	s.logger.Debug("New server", "component", "polling", "mac", srv.Mac)
	if err := s.eventLog.AppendEvent(context.Background(), event.HostPoll, srv, "", "", nil); err != nil {
		s.logger.Error("Failed to record host poll event", "component", "polling", "mac", srv.Mac, "err", err)
	}

	return nil, RetryAction
}

func resolveBootTarget(resolver *mappings.Resolver, request mappings.ResolveRequest,
	baseURL string, srv server.Server) (mappings.ResolveResult, server.Server, error) {
	result, err := resolver.Resolve(request)
	if err != nil || !result.HasTarget() {
		return result, srv, err
	}

	request.GeneratedParams = generatedBootParams(result.Params, result.Provisioning.Network.Hostname, baseURL, result.Target.Environment, srv, result.MatchType)
	result, err = resolver.Resolve(request)
	if err != nil {
		return mappings.ResolveResult{}, srv, err
	}
	if hostname, ok := result.Params["hostname"].(string); ok {
		srv.Hostname = hostname
	}
	return result, srv, nil
}

func generatedBootParams(params map[string]any, structuredHostname, baseURL, envName string, srv server.Server, matchType mappings.MatchType) map[string]any {
	generated := map[string]any{
		"baseURL": utils.BaseURLforEnvName(baseURL, envName),
	}
	if _, hasLegacyHostname := params["hostname"]; !hasLegacyHostname && structuredHostname != "" {
		generated["hostname"] = structuredHostname
		return generated
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

func copyParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	copied := make(map[string]any, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return copied
}

func setHostName(params map[string]any, mac string) {
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
	return genStartScript(logger, baseURL)
}

func genStartScript(logger log.Logger, baseURL string) string {
	variablesMap := map[string]any{}
	parsedTemplate := &bytes.Buffer{}

	tmpl, err := template.New("retry").Parse(startScript)
	if err != nil {
		logger.Info("Error parsing start template", "component", "polling")
		panic(err)
	}

	variablesMap["baseURL"] = baseURL
	err = tmpl.Execute(parsedTemplate, variablesMap)
	if err != nil {
		logger.Info("Error executing start template", "component", "polling")
		panic(err)
	}

	return parsedTemplate.String()
}

func (s *Service) genBootScript(scriptName, envName string, params map[string]any, users map[string]mappings.ResolvedUser, provisioning mappings.ProvisioningConfig) string {
	text, err := s.templateRenderer.RenderTemplate(scriptName, mappings.ParamsWithProvisioning(params, users, provisioning), envName)
	if err != nil {
		panic(err)
	}
	return text
}

func (s *Service) genRetryScript(mac string) string {
	variablesMap := map[string]any{}
	parsedTemplate := &bytes.Buffer{}

	tmpl, err := template.New("retry").Parse(retryScript)
	if err != nil {
		s.logger.Info("Error parsing retry template", "component", "polling", "mac", mac)
		panic(err)
	}

	variablesMap["baseURL"] = s.baseURL
	variablesMap["macAddress"] = utils.MacColonToDash(mac)
	err = tmpl.Execute(parsedTemplate, variablesMap)
	if err != nil {
		s.logger.Info("Error executing retry template", "component", "polling", "mac", mac)
		panic(err)
	}

	return parsedTemplate.String()
}
