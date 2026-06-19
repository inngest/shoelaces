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
	"database/sql"
	"errors"
	"fmt"
	"text/template"
	"time"

	"github.com/inngest/shoelaces/bootsession"
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
	serverStates     server.StateStore
	resolver         *mappings.Resolver
	eventLog         *event.Log
	templateRenderer *templates.ShoelacesTemplates
	bootSessions     *bootsession.Store
	baseURL          string
}

func NewService(logger log.Logger, serverStates server.StateStore, resolver *mappings.Resolver, eventLog *event.Log, templateRenderer *templates.ShoelacesTemplates, baseURL string) *Service {
	return &Service{
		logger:           logger.With("component", "polling"),
		serverStates:     serverStates,
		resolver:         resolver,
		eventLog:         eventLog,
		templateRenderer: templateRenderer,
		baseURL:          baseURL,
	}
}

// WithBootSessions enables boot/config references for rendered boot scripts.
func (s *Service) WithBootSessions(store *bootsession.Store) *Service {
	s.bootSessions = store
	return s
}

// ListServers returns the hosts currently waiting for manual target selection.
func (s *Service) ListServers() (server.Servers, error) {
	return s.serverStates.ListWaiting(context.Background())
}

// StartScript renders the initial iPXE polling script.
func (s *Service) StartScript() string {
	return genStartScript(s.logger, s.baseURL)
}

// UpdateTarget stores a manually selected target for a polling host.
func (s *Service) UpdateTarget(srv server.Server, targetName string, environment string, params map[string]any) (inputErr bool, err error) {
	logger := s.logger.With("mac", srv.Mac)
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

	state, err := s.serverStates.GetState(context.Background(), srv.Mac)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, errors.New("MAC is not in the booting state")
		}
		return false, err
	}
	if state == nil {
		return true, errors.New("MAC is not in the booting state")
	}

	bootServer := state.Server
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

	logger.Debug("Setting server override", "target", targetName, "script", result.Target.Script, "environment", result.Target.Environment, "hostname", resolvedServer.Hostname, "params", utils.RedactParams(result.Params))
	if err := s.eventLog.AppendEvent(context.Background(), event.UserSelection, resolvedServer, "", result.Target.Script, nil); err != nil {
		return false, err
	}
	state.Server = resolvedServer
	state.Target = result.Target.Script
	state.Environment = result.Target.Environment
	state.Params = result.Params
	state.Users = result.Users
	state.Provisioning = result.Provisioning
	state.LastAccess = int(time.Now().UTC().Unix())
	if err := s.serverStates.SaveState(context.Background(), state); err != nil {
		return false, err
	}
	return false, nil
}

// Poll resolves and renders the next script for a booting host.
func (s *Service) Poll(srv server.Server) (scriptText string, err error) {
	logger := s.logger.With("mac", srv.Mac)
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
		logger.Debug("Host found", "where", result.MatchType, "host", srv.Hostname, "ip", srv.IP)
		if err := s.eventLog.AppendEvent(context.Background(), event.HostBoot, resolvedServer, bootTypeForMatch(result.MatchType), result.Target.Script, result.Params); err != nil {
			return "", err
		}
		return s.genBootScript(resolvedServer, result.Target.Script, result.Target.Environment, result.Params, result.Users, result.Provisioning)
	}

	logger.Debug("Host needs manual target selection", "where", result.MatchType, "ip", srv.IP)
	return s.manualAction(srv, targetOptions(result.AllowedTargets))
}

// ListServers provides a list of the servers that tried to boot
// but did not match the hostname regex or network mappings.
func ListServers(serverStates *server.States) server.Servers {
	servers, err := serverStates.ListWaiting(context.Background())
	if err != nil {
		return nil
	}
	return servers
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
	logger := s.logger.With("mac", srv.Mac)

	script, action, err := s.chooseManualAction(srv, allowedTargets)
	if err != nil {
		return "", err
	}
	logger.Debug("Manual action selected", "target-script-name", script, "action", action)

	switch action {
	case BootAction:
		setHostName(script.Params, srv.Mac)
		srv.Hostname = script.Params["hostname"].(string)
		if err := s.eventLog.AppendEvent(context.Background(), event.HostBoot, srv, event.ManualBoot, script.Name, script.Params); err != nil {
			return "", err
		}
		return s.genBootScript(srv, script.Name, script.Environment, script.Params, script.Users, script.Provisioning)

	case RetryAction:
		return s.genRetryScript(srv.Mac), nil

	case TimeoutAction:
		return timeoutScript, nil

	default:
		logger.Info("Unknown action")
		return "", fmt.Errorf("%s", "Unknown action")
	}
}

func (s *Service) chooseManualAction(srv server.Server, allowedTargets []server.TargetOption) (*mappings.Script, ManualAction, error) {
	logger := s.logger.With("mac", srv.Mac)

	m, err := s.serverStates.GetState(context.Background(), srv.Mac)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("Failed to load manual state", "err", err)
		return nil, TimeoutAction, err
	}
	if m != nil {
		if m.Target != server.InitTarget {
			if err := s.serverStates.DeleteState(context.Background(), srv.Mac); err != nil {
				logger.Error("Failed to delete selected server state", "err", err)
				return nil, TimeoutAction, err
			}
			logger.Debug("Server boot")
			return &mappings.Script{
				Name:         m.Target,
				Environment:  m.Environment,
				Params:       m.Params,
				Users:        m.Users,
				Provisioning: m.Provisioning}, BootAction, nil
		} else if m.Retry <= maxRetry {
			m.Retry++
			m.LastAccess = int(time.Now().UTC().Unix())
			if err := s.serverStates.SaveState(context.Background(), m); err != nil {
				logger.Error("Failed to persist retry state", "err", err)
				return nil, TimeoutAction, err
			}
			logger.Debug("Retrying reboot")
			return nil, RetryAction, nil
		} else {
			if err := s.serverStates.DeleteState(context.Background(), srv.Mac); err != nil {
				logger.Error("Failed to delete timed-out server state", "err", err)
				return nil, TimeoutAction, err
			}
			logger.Debug("Timing out server")
			return nil, TimeoutAction, nil
		}
	}

	if err := s.serverStates.QueueServer(context.Background(), srv, allowedTargets); err != nil {
		logger.Error("Failed to persist new server state", "err", err)
		return nil, TimeoutAction, err
	}
	logger.Debug("New server")
	if err := s.eventLog.AppendEvent(context.Background(), event.HostPoll, srv, "", "", nil); err != nil {
		logger.Error("Failed to record host poll event", "err", err)
	}

	return nil, RetryAction, nil
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

func (s *Service) genBootScript(srv server.Server, scriptName, envName string, params map[string]any, users map[string]mappings.ResolvedUser, provisioning mappings.ProvisioningConfig) (string, error) {
	renderParams := copyParams(params)
	if s.bootSessions != nil {
		ref, err := s.bootSessions.Create(context.Background(), bootsession.Snapshot{
			Server:       srv,
			Target:       scriptName,
			Environment:  envName,
			Params:       params,
			Users:        users,
			Provisioning: provisioning,
		})
		if err != nil {
			return "", err
		}
		s.logger.Info("Created boot reference", "component", "polling", "ref", ref, "mac", srv.Mac, "target", scriptName, "environment", envName)
		bootsession.ApplyReferenceParams(renderParams, ref)
	}
	text, err := s.templateRenderer.RenderTemplate(scriptName, mappings.ParamsWithProvisioning(renderParams, users, provisioning), envName)
	if err != nil {
		return "", err
	}
	return text, nil
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
