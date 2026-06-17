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

package mappings

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
)

// EnvLookup reads an environment-backed parameter value.
type EnvLookup func(string) (string, bool)

// MatchType identifies which mapping rule selected or constrained a boot.
type MatchType string

const (
	// MatchManual means the caller explicitly selected a target.
	MatchManual MatchType = "manual"
	// MatchMAC means an exact MAC address mapping matched the host.
	MatchMAC MatchType = "mac"
	// MatchIP means an exact IP address mapping matched the host.
	MatchIP MatchType = "ip"
	// MatchHostname means a hostname regular expression matched the host.
	MatchHostname MatchType = "hostname"
	// MatchNetwork means a CIDR network mapping matched the host IP.
	MatchNetwork MatchType = "network"
	// MatchUnmatched means no mapping matched and the host needs manual choice.
	MatchUnmatched MatchType = "unmatched"
)

// ResolveRequest is the host identity and optional manual selection to resolve.
type ResolveRequest struct {
	// Mac is the host MAC address using either colon or dash separators.
	Mac string
	// IP is the host IPv4 or IPv6 address.
	IP string
	// Hostname is the resolved or request-provided host name.
	Hostname string
	// ManualTarget is the explicit target name selected by a user/operator.
	ManualTarget string
	// Params contains request or manual form parameters supplied by the caller.
	Params map[string]any
	// GeneratedParams contains values produced by Shoelaces, such as hostname
	// and baseURL. These have the highest merge precedence.
	GeneratedParams map[string]any
	// EnvLookup resolves explicit { env: VAR } parameter references. When nil,
	// os.LookupEnv is used.
	EnvLookup EnvLookup
}

// TargetCandidate is an immutable snapshot of a named boot target.
type TargetCandidate struct {
	// Name is the target key from mappings.yaml.
	Name string
	// Script is the dynamic iPXE template name to render.
	Script string
	// Label is optional display text for manual selection.
	Label string
	// Environment selects a Shoelaces env override when rendering templates.
	Environment string
	// Params is a copied target parameter map.
	Params map[string]any
	// Users is a copied target user map.
	Users map[string]UserConfig
}

// ResolveResult describes the selected target or the manual choice set.
type ResolveResult struct {
	// MatchType records the highest-precedence rule that matched the request.
	MatchType MatchType
	// TargetName is set when resolution selected a concrete target.
	TargetName string
	// Target is the selected target snapshot. It is zero-valued when the host
	// needs manual target selection.
	Target TargetCandidate
	// AllowedTargets contains the target choices available to a manual selector.
	AllowedTargets []TargetCandidate
	// MappingParams is a copied parameter map from the matched mapping rule.
	MappingParams map[string]any
	// Params is the fully merged and normalized runtime template parameter map.
	Params map[string]any
	// Users is the fully merged and normalized runtime account map.
	Users map[string]ResolvedUser
	// RequiresManualSelection is true when no default target was selected.
	RequiresManualSelection bool
}

// HasTarget reports whether the resolver selected a concrete boot target.
func (r ResolveResult) HasTarget() bool {
	return r.TargetName != ""
}

// AllowedTargetNames returns the allowed target names in their configured order.
func (r ResolveResult) AllowedTargetNames() []string {
	names := make([]string, 0, len(r.AllowedTargets))
	for _, target := range r.AllowedTargets {
		names = append(names, target.Name)
	}
	return names
}

// Resolver compiles a Mappings object for deterministic host target selection.
type Resolver struct {
	// defaults stores copied global default params from mappings.yaml.
	defaults map[string]any
	// defaultUsers stores copied global user defaults from mappings.yaml.
	defaultUsers map[string]UserConfig
	// targets stores copied target definitions by name.
	targets map[string]Target
	// targetOrder gives deterministic ordering for unrestricted manual choices.
	targetOrder []string
	// macMaps contains exact MAC mapping policies in file order.
	macMaps []compiledMACMap
	// ipMaps contains exact IP mapping policies in file order.
	ipMaps []compiledIPMap
	// hostnameMaps contains compiled hostname regex policies in file order.
	hostnameMaps []compiledHostnameMap
	// networkMaps contains parsed CIDR policies in file order.
	networkMaps []compiledNetworkMap
}

type compiledPolicy struct {
	defaultTarget string
	targets       []string
	params        map[string]any
	users         map[string]UserConfig
}

type compiledMACMap struct {
	mac    string
	policy compiledPolicy
}

type compiledIPMap struct {
	ip     net.IP
	policy compiledPolicy
}

type compiledHostnameMap struct {
	hostname *regexp.Regexp
	policy   compiledPolicy
}

type compiledNetworkMap struct {
	network *net.IPNet
	policy  compiledPolicy
}

// NewResolver validates and compiles mappings for target resolution.
func NewResolver(mappings *Mappings) (*Resolver, error) {
	if mappings == nil {
		mappings = defaultMappings()
	}
	if err := validateMappings(mappings); err != nil {
		return nil, err
	}

	resolver := &Resolver{
		defaults:     copyParamMap(mappings.Defaults.Params),
		defaultUsers: copyUserConfigMap(mappings.Defaults.Users),
		targets:      make(map[string]Target, len(mappings.Targets)),
	}
	for name, target := range mappings.Targets {
		resolver.targets[name] = copyTarget(target)
		resolver.targetOrder = append(resolver.targetOrder, name)
	}
	sort.Strings(resolver.targetOrder)

	for _, mapping := range mappings.MacMaps {
		mac, err := net.ParseMAC(mapping.Mac)
		if err != nil {
			return nil, err
		}
		resolver.macMaps = append(resolver.macMaps, compiledMACMap{
			mac:    mac.String(),
			policy: compilePolicy(mapping.DefaultTarget, mapping.Targets, mapping.Params, mapping.Users),
		})
	}

	for _, mapping := range mappings.IPMaps {
		resolver.ipMaps = append(resolver.ipMaps, compiledIPMap{
			ip:     net.ParseIP(mapping.IP),
			policy: compilePolicy(mapping.DefaultTarget, mapping.Targets, mapping.Params, mapping.Users),
		})
	}

	for _, mapping := range mappings.HostnameMaps {
		hostname, err := regexp.Compile(mapping.Hostname)
		if err != nil {
			return nil, err
		}
		resolver.hostnameMaps = append(resolver.hostnameMaps, compiledHostnameMap{
			hostname: hostname,
			policy:   compilePolicy(mapping.DefaultTarget, mapping.Targets, mapping.Params, mapping.Users),
		})
	}

	for _, mapping := range mappings.NetworkMaps {
		_, network, err := net.ParseCIDR(mapping.Network)
		if err != nil {
			return nil, err
		}
		resolver.networkMaps = append(resolver.networkMaps, compiledNetworkMap{
			network: network,
			policy:  compilePolicy(mapping.DefaultTarget, mapping.Targets, mapping.Params, mapping.Users),
		})
	}

	return resolver, nil
}

// Resolve selects a target using manual, MAC, IP, hostname, then network
// precedence. When no default target is available, it returns allowed choices.
func (r *Resolver) Resolve(request ResolveRequest) (ResolveResult, error) {
	if request.ManualTarget != "" {
		return r.resolveManual(request)
	}

	policy, matchType := r.findPolicy(request)
	if policy == nil {
		return ResolveResult{
			MatchType:               MatchUnmatched,
			AllowedTargets:          r.allTargets(),
			RequiresManualSelection: true,
		}, nil
	}
	result := ResolveResult{
		MatchType:      matchType,
		AllowedTargets: r.targetsByName(policy.targets),
		MappingParams:  copyParamMap(policy.params),
	}
	if policy.defaultTarget == "" {
		result.RequiresManualSelection = true
		return result, nil
	}

	target, err := r.targetByName(policy.defaultTarget)
	if err != nil {
		return ResolveResult{}, err
	}
	result.TargetName = policy.defaultTarget
	result.Target = target
	result.Params, err = r.resolveParams(target, result.MappingParams, request)
	if err != nil {
		return ResolveResult{}, err
	}
	result.Users, err = r.resolveUsers(target.Users, policy.users, request)
	if err != nil {
		return ResolveResult{}, err
	}
	return result, nil
}

func (r *Resolver) resolveManual(request ResolveRequest) (ResolveResult, error) {
	policy, _ := r.findPolicy(request)
	allowedTargets := r.allTargets()
	mappingParams := map[string]any(nil)
	if policy != nil {
		if !stringInSlice(request.ManualTarget, policy.targets) {
			return ResolveResult{}, fmt.Errorf("manual target %q is not allowed", request.ManualTarget)
		}
		allowedTargets = r.targetsByName(policy.targets)
		mappingParams = copyParamMap(policy.params)
	}

	target, err := r.targetByName(request.ManualTarget)
	if err != nil {
		return ResolveResult{}, err
	}
	result := ResolveResult{
		MatchType:      MatchManual,
		TargetName:     request.ManualTarget,
		Target:         target,
		AllowedTargets: allowedTargets,
		MappingParams:  mappingParams,
	}
	result.Params, err = r.resolveParams(target, mappingParams, request)
	if err != nil {
		return ResolveResult{}, err
	}
	var policyUsers map[string]UserConfig
	if policy != nil {
		policyUsers = policy.users
	}
	result.Users, err = r.resolveUsers(target.Users, policyUsers, request)
	if err != nil {
		return ResolveResult{}, err
	}
	return result, nil
}

func (r *Resolver) findPolicy(request ResolveRequest) (*compiledPolicy, MatchType) {
	if policy := r.findMACPolicy(request.Mac); policy != nil {
		return policy, MatchMAC
	}
	if policy := r.findIPPolicy(request.IP); policy != nil {
		return policy, MatchIP
	}
	if policy := r.findHostnamePolicy(request.Hostname); policy != nil {
		return policy, MatchHostname
	}
	if policy := r.findNetworkPolicy(request.IP); policy != nil {
		return policy, MatchNetwork
	}
	return nil, MatchUnmatched
}

func (r *Resolver) findMACPolicy(mac string) *compiledPolicy {
	parsed, err := parseRequestMAC(mac)
	if err != nil {
		return nil
	}
	for i := range r.macMaps {
		if r.macMaps[i].mac == parsed {
			return &r.macMaps[i].policy
		}
	}
	return nil
}

func (r *Resolver) findIPPolicy(ip string) *compiledPolicy {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil
	}
	for i := range r.ipMaps {
		if r.ipMaps[i].ip.Equal(parsed) {
			return &r.ipMaps[i].policy
		}
	}
	return nil
}

func (r *Resolver) findHostnamePolicy(hostname string) *compiledPolicy {
	for i := range r.hostnameMaps {
		if r.hostnameMaps[i].hostname.MatchString(hostname) {
			return &r.hostnameMaps[i].policy
		}
	}
	return nil
}

func (r *Resolver) findNetworkPolicy(ip string) *compiledPolicy {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil
	}
	for i := range r.networkMaps {
		if r.networkMaps[i].network.Contains(parsed) {
			return &r.networkMaps[i].policy
		}
	}
	return nil
}

func (r *Resolver) targetByName(name string) (TargetCandidate, error) {
	target, ok := r.targets[name]
	if !ok {
		return TargetCandidate{}, fmt.Errorf("target %q not found", name)
	}
	return targetCandidate(name, target), nil
}

func (r *Resolver) targetsByName(names []string) []TargetCandidate {
	targets := make([]TargetCandidate, 0, len(names))
	for _, name := range names {
		target, ok := r.targets[name]
		if !ok {
			continue
		}
		targets = append(targets, targetCandidate(name, target))
	}
	return targets
}

func (r *Resolver) allTargets() []TargetCandidate {
	return r.targetsByName(r.targetOrder)
}

func (r *Resolver) resolveParams(target TargetCandidate, mappingParams map[string]any, request ResolveRequest) (map[string]any, error) {
	merged := make(map[string]any)
	mergeParamMap(merged, r.defaults)
	mergeParamMap(merged, target.Params)
	mergeParamMap(merged, mappingParams)
	mergeParamMap(merged, request.Params)
	mergeParamMap(merged, request.GeneratedParams)

	lookup := request.EnvLookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	for key, value := range merged {
		resolved, err := resolveParamValue(key, value, lookup)
		if err != nil {
			return nil, err
		}
		merged[key] = resolved
	}
	return merged, nil
}

func (r *Resolver) resolveUsers(targetUsers map[string]UserConfig, mappingUsers map[string]UserConfig, request ResolveRequest) (map[string]ResolvedUser, error) {
	merged := make(map[string]UserConfig)
	mergeUserConfigMap(merged, r.defaultUsers)
	mergeUserConfigMap(merged, targetUsers)
	mergeUserConfigMap(merged, mappingUsers)

	lookup := request.EnvLookup
	if lookup == nil {
		lookup = os.LookupEnv
	}

	resolved := make(map[string]ResolvedUser)
	primaryUsers := make([]string, 0)
	for name, user := range merged {
		if boolValue(user.Absent) {
			continue
		}
		resolvedUser, err := resolveUser(name, user, lookup)
		if err != nil {
			return nil, err
		}
		if name != "root" && resolvedUser.Primary {
			primaryUsers = append(primaryUsers, name)
		}
		resolved[name] = resolvedUser
	}
	sort.Strings(primaryUsers)
	if len(primaryUsers) > 1 {
		return nil, fmt.Errorf("multiple primary non-root users configured: %v", primaryUsers)
	}
	return resolved, nil
}

func mergeParamMap(dst map[string]any, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}

func resolveParamValue(key string, value any, lookup EnvLookup) (any, error) {
	envRef, ok, err := envReference(value)
	if err != nil {
		return nil, fmt.Errorf("parameter %q: %w", key, err)
	}
	if !ok {
		return value, nil
	}
	resolved, found := lookup(envRef)
	if !found {
		return nil, fmt.Errorf("parameter %q references missing environment variable %q", key, envRef)
	}
	return resolved, nil
}

func envReference(value any) (env string, ok bool, err error) {
	switch typed := value.(type) {
	case map[string]any:
		return envReferenceFromMap(typed)
	default:
		return "", false, nil
	}
}

func envReferenceFromMap(value map[string]any) (string, bool, error) {
	envValue, ok := value["env"]
	if !ok {
		return "", false, nil
	}
	if len(value) != 1 {
		return "", false, fmt.Errorf(`environment reference must only contain "env"`)
	}
	env, ok := envValue.(string)
	if !ok || env == "" {
		return "", false, fmt.Errorf(`environment reference "env" must be a non-empty string`)
	}
	return env, true, nil
}

func compilePolicy(defaultTarget string, targets []string, params map[string]any, users map[string]UserConfig) compiledPolicy {
	return compiledPolicy{
		defaultTarget: defaultTarget,
		targets:       append([]string(nil), targets...),
		params:        copyParamMap(params),
		users:         copyUserConfigMap(users),
	}
}

func targetCandidate(name string, target Target) TargetCandidate {
	return TargetCandidate{
		Name:        name,
		Script:      target.Script,
		Label:       target.Label,
		Environment: target.Environment,
		Params:      copyParamMap(target.Params),
		Users:       copyUserConfigMap(target.Users),
	}
}

func copyTarget(target Target) Target {
	target.Params = copyParamMap(target.Params)
	target.Users = copyUserConfigMap(target.Users)
	return target
}

func copyParamMap(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	copied := make(map[string]any, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return copied
}

func parseRequestMAC(mac string) (string, error) {
	parsed, err := net.ParseMAC(mac)
	if err == nil {
		return parsed.String(), nil
	}
	for i := range mac {
		if mac[i] == '-' {
			parsed, err = net.ParseMAC(replaceMACDashes(mac))
			if err != nil {
				return "", err
			}
			return parsed.String(), nil
		}
	}
	return "", err
}

func replaceMACDashes(mac string) string {
	buf := []byte(mac)
	for i, b := range buf {
		if b == '-' {
			buf[i] = ':'
		}
	}
	return string(buf)
}
