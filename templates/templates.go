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

package templates

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"

	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/internal/validation"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/mappings"
	"github.com/inngest/shoelaces/utils"
)

const defaultEnvironment = "default"

// ShoelacesTemplates holds the core attributes for handling the dyanmic configurations
// in Shoelaces.
type ShoelacesTemplates struct {
	logger       log.Logger
	envTemplates map[string]shoelacesTemplateEnvironment
	dataDir      string
	envDir       string
	tplExt       string
}

type shoelacesTemplateEnvironment struct {
	templateObj      *template.Template
	templateVars     map[string][]string
	templateRefs     map[string][]string
	templateEmbedded map[string]bool
	templateBodies   map[string]string
}

// New creates and initializes a new ShoelacesTemplates instance a returns a pointer to
// it.
func New(logger log.Logger) *ShoelacesTemplates {
	if logger == nil {
		logger = log.MakeLogger(io.Discard)
	}
	logger = logger.With("component", "template")
	e := make(map[string]shoelacesTemplateEnvironment)
	e[defaultEnvironment] = shoelacesTemplateEnvironment{
		templateObj:      template.New("").Funcs(templateFuncMap()),
		templateVars:     make(map[string][]string),
		templateRefs:     make(map[string][]string),
		templateEmbedded: make(map[string]bool),
		templateBodies:   make(map[string]string),
	}
	return &ShoelacesTemplates{logger: logger, envTemplates: e}
}

func (s *ShoelacesTemplates) checkAddEnvironment(environment string) {
	if _, ok := s.envTemplates[environment]; !ok {
		c, e := s.envTemplates[defaultEnvironment].templateObj.Clone()
		if e != nil {
			s.logger.Error("Template for environment already executed", "environment", environment)
			os.Exit(1)
		}
		s.envTemplates[environment] = shoelacesTemplateEnvironment{
			templateObj: c,
			// Environment overrides inherit the default index, then replace entries
			// for any templates they override.
			templateVars:     cloneTemplateIndex(s.envTemplates[defaultEnvironment].templateVars),
			templateRefs:     cloneTemplateIndex(s.envTemplates[defaultEnvironment].templateRefs),
			templateEmbedded: cloneTemplateEmbeddedIndex(s.envTemplates[defaultEnvironment].templateEmbedded),
			templateBodies:   cloneTemplateBodyIndex(s.envTemplates[defaultEnvironment].templateBodies),
		}
	}
}

func (s *ShoelacesTemplates) addTemplate(path string, environment string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return s.addTemplateContent(path, content, environment, false)
}

func (s *ShoelacesTemplates) addTemplateContent(source string, content []byte, environment string, embedded bool) error {
	s.checkAddEnvironment(environment)
	sourceName := filepath.Base(source)
	parsed, err := template.New(sourceName).Funcs(templateFuncMap()).Parse(string(content))
	if err != nil {
		return err
	}

	templateEnv := s.envTemplates[environment]
	for _, parsedTemplate := range parsed.Templates() {
		if parsedTemplate.Tree == nil || parsedTemplate.Root == nil {
			continue
		}
		if parsedTemplate.Name() == sourceName && strings.TrimSpace(parsedTemplate.Root.String()) == "" {
			continue
		}
		if _, err := templateEnv.templateObj.AddParseTree(parsedTemplate.Name(), parsedTemplate.Tree); err != nil {
			return err
		}
		templateEnv.templateVars[parsedTemplate.Name()], templateEnv.templateRefs[parsedTemplate.Name()] = extractTemplateInfo(parsedTemplate.Root)
		body := parsedTemplate.Root.String()
		templateEnv.templateEmbedded[parsedTemplate.Name()] = embedded || (templateEnv.templateEmbedded[parsedTemplate.Name()] && templateEnv.templateBodies[parsedTemplate.Name()] == body)
		templateEnv.templateBodies[parsedTemplate.Name()] = body
	}
	return nil
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"shquote": shellQuote,
	}
}

// shellQuote is intentionally small and POSIX-sh compatible. Installer
// templates use it for values that must become one shell word, including the
// LUKS passphrase temp-file write used before TPM enrollment.
func shellQuote(value any) string {
	text := fmt.Sprint(value)
	if text == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(text, "'", "'\\''") + "'"
}

func cloneTemplateIndex(index map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(index))
	for name, values := range index {
		cloned[name] = append([]string(nil), values...)
	}
	return cloned
}

func cloneTemplateEmbeddedIndex(index map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(index))
	for name, embedded := range index {
		cloned[name] = embedded
	}
	return cloned
}

func cloneTemplateBodyIndex(index map[string]string) map[string]string {
	cloned := make(map[string]string, len(index))
	for name, body := range index {
		cloned[name] = body
	}
	return cloned
}

func extractTemplateInfo(root parse.Node) ([]string, []string) {
	// Keep direct variables and template references separate. References are
	// resolved after parsing so disk overrides and later partial definitions
	// participate in ListVariables and missing-variable errors.
	var variables []string
	var refs []string
	walkTemplateNode(root, func(variable string) {
		if variable != "" && !utils.StringInSlice(variable, variables) {
			variables = append(variables, variable)
		}
	}, func(ref string) {
		if ref != "" && !utils.StringInSlice(ref, refs) {
			refs = append(refs, ref)
		}
	})
	return variables, refs
}

func walkTemplateNode(node parse.Node, addVariable func(string), addRef func(string)) {
	if node == nil {
		return
	}
	// Optional branches and pipes can arrive as typed nil parse nodes.
	value := reflect.ValueOf(node)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return
	}
	switch n := node.(type) {
	case *parse.ActionNode:
		walkTemplateNode(n.Pipe, addVariable, addRef)
	case *parse.BranchNode:
		walkTemplateNode(n.Pipe, addVariable, addRef)
		walkTemplateNode(n.List, addVariable, addRef)
		walkTemplateNode(n.ElseList, addVariable, addRef)
	case *parse.ChainNode:
		walkTemplateNode(n.Node, addVariable, addRef)
		addVariable(normalizeVariableName(strings.Join(n.Field, ".")))
	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkTemplateNode(arg, addVariable, addRef)
		}
	case *parse.FieldNode:
		addVariable(normalizeVariableName(strings.Join(n.Ident, ".")))
	case *parse.IfNode:
		walkTemplateNode(n.Pipe, addVariable, addRef)
		walkTemplateNode(n.List, addVariable, addRef)
		walkTemplateNode(n.ElseList, addVariable, addRef)
	case *parse.ListNode:
		for _, child := range n.Nodes {
			walkTemplateNode(child, addVariable, addRef)
		}
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			walkTemplateNode(cmd, addVariable, addRef)
		}
	case *parse.RangeNode:
		walkTemplateNode(n.Pipe, addVariable, addRef)
		walkTemplateNode(n.List, addVariable, addRef)
		walkTemplateNode(n.ElseList, addVariable, addRef)
	case *parse.TemplateNode:
		// Template references are followed during variable listing so partials
		// can be parsed or overridden independently of their callers.
		addRef(n.Name)
		walkTemplateNode(n.Pipe, addVariable, addRef)
	case *parse.WithNode:
		walkTemplateNode(n.Pipe, addVariable, addRef)
		walkTemplateNode(n.List, addVariable, addRef)
		walkTemplateNode(n.ElseList, addVariable, addRef)
	}
}

func normalizeVariableName(variable string) string {
	variable = strings.TrimSpace(variable)
	if variable == "" {
		return ""
	}
	fields := strings.Fields(variable)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimPrefix(fields[0], ".")
}

func (s *ShoelacesTemplates) getEnvFromPath(path string) string {
	envPath := filepath.Join(s.dataDir, s.envDir)
	if strings.HasPrefix(path, envPath) {
		return strings.Split(strings.TrimPrefix(path, envPath), "/")[1]
	}
	return defaultEnvironment
}

// ParseTemplates travels the dataDir and loads in an internal structure
// all the templates found.
func (s *ShoelacesTemplates) ParseTemplates(dataDir string, envDir string, envs []string, tplExt string) {
	s.dataDir = dataDir
	s.envDir = envDir
	s.tplExt = tplExt

	s.logger.Debug("Template parsing started", "dir", dataDir)
	s.parseEmbeddedProvisioningTemplates()

	tplScannerDefault := func(p string, info os.FileInfo, err error) error {
		if strings.HasPrefix(p, path.Join(dataDir, envDir)) {
			return err
		}
		if strings.HasSuffix(p, tplExt) {
			s.logger.Info("Parsing file", "file", p)
			if err := s.addTemplate(p, defaultEnvironment); err != nil {
				s.logger.Error("Template parse failed", "err", err)
				os.Exit(1)
			}
		}
		return err
	}

	tplScannerOverride := func(p string, info os.FileInfo, err error) error {
		if strings.HasSuffix(p, tplExt) {
			env := s.getEnvFromPath(p)
			s.logger.Info("Parsing override", "environment", env, "file", p)

			if err := s.addTemplate(p, env); err != nil {
				s.logger.Error("Template override parse failed", "err", err)
				os.Exit(1)
			}
		}
		return err
	}

	if err := filepath.Walk(dataDir, tplScannerDefault); err != nil {
		panic(err)
	}
	s.logger.Info("Parsing override files", "dir", path.Join(dataDir, envDir))
	if err := filepath.Walk(path.Join(dataDir, envDir), tplScannerOverride); err != nil {
		s.logger.Info("No overrides found")
	}
	s.logger.Debug("Parsing ended")
}

func (s *ShoelacesTemplates) parseEmbeddedProvisioningTemplates() {
	defaults := shoelaces.ProvisioningDefaultsFS()
	err := fs.WalkDir(defaults, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(p, s.tplExt) {
			s.logger.Info("Parsing embedded provisioning file", "file", p)
			content, err := fs.ReadFile(defaults, p)
			if err != nil {
				return err
			}
			return s.addTemplateContent(p, content, defaultEnvironment, true)
		}
		return nil
	})
	if err != nil {
		s.logger.Error("Embedded provisioning template parse failed", "err", err)
		os.Exit(1)
	}
}

// RenderTemplate receives a name and a map of parameters, among other
// arguments, and returns the rendered template. It's aware of the
// environment, in case of any.
func (s *ShoelacesTemplates) RenderTemplate(configName string, paramMap map[string]interface{}, envName string) (string, error) {
	if envName == "" {
		envName = defaultEnvironment
	}
	if paramMap == nil {
		paramMap = map[string]interface{}{}
	}
	provisioning, _ := paramMap["provisioning"].(mappings.ProvisioningConfig)
	paramMap = mappings.ParamsWithProvisioning(paramMap, nil, provisioning)
	paramMap, err := s.withInstallerExtra(paramMap, envName, configName)
	if err != nil {
		s.logger.Info("Failed to render installer extra", "action", "render-installer-extra", "err", err)
		return "", err
	}
	if err := validateRenderParams(configName, paramMap, s.isEmbeddedTemplate(configName, envName)); err != nil {
		s.logger.Info("Invalid template parameters", "action", "validate-template-params", "template", configName, "err", err)
		return "", err
	}
	s.logger.Info("Template request", "action", "template-request", "template", configName, "env", envName, "parameters", utils.RedactedMapToString(paramMap))

	requiredVariables := s.envTemplates[envName].listVariables(configName)

	var b bytes.Buffer
	err = s.envTemplates[envName].templateObj.ExecuteTemplate(&b, configName, paramMap)
	// Fall back to default template in case this is non default environment
	// XXX: this is temporary and will be simplified to reduce the code duplication
	if err != nil && envName != defaultEnvironment {
		requiredVariables = s.envTemplates[defaultEnvironment].listVariables(configName)
		err = s.envTemplates[defaultEnvironment].templateObj.ExecuteTemplate(&b, configName, paramMap)
	}
	if err != nil {
		s.logger.Info("Failed to render template", "action", "render-template", "err", err)
		return "", err
	}
	r := b.String()
	if strings.Contains(r, "<no value>") {
		missingVariables := ""
		for _, requiredVariable := range requiredVariables {
			if !utils.KeyInMap(requiredVariable, paramMap) {
				if len(missingVariables) > 0 {
					missingVariables += ", "
				}
				missingVariables += requiredVariable
			}
		}
		s.logger.Info("Missing variables in request", "variables", missingVariables)
		return "", errors.New("Missing variables in request: " + missingVariables)
	}

	return r, nil
}

func validateRenderParams(configName string, paramMap map[string]interface{}, embeddedTemplate bool) error {
	switch configName {
	case "preseed/debian", "generated/debian/luks-tpm-setup.sh", "generated/plain/luks-tpm.passphrase":
		return validateDebianPreseedParams(paramMap)
	default:
		if isStorageEncryptionEnabled(paramMap) && isUnsupportedEncryptedInstallerTemplate(configName) && embeddedTemplate && !hasInstallerExtraTemplate(paramMap, configName) {
			return fmt.Errorf("%s does not support structured storage encryption; storage.encryption is supported only by preseed/debian in the embedded templates. Use installer.extraTemplate or a full template override for non-Debian encrypted installs", configName)
		}
		return nil
	}
}

func isUnsupportedEncryptedInstallerTemplate(configName string) bool {
	switch configName {
	case "centos.ipxe", "centos.ks", "coreos.ipxe", "cloudconfig-coreos", "preseed/storage", "preseed/ubuntu-minimal", "storage.ipxe", "ubuntu-minimal.ipxe":
		return true
	}
	if strings.HasPrefix(configName, "preseed/") && configName != "preseed/debian" {
		return true
	}
	if strings.HasSuffix(configName, ".ks") {
		return true
	}
	if strings.HasPrefix(configName, "cloudconfig") || strings.HasPrefix(configName, "cloud-config/") {
		return true
	}
	return false
}

func hasInstallerExtraTemplate(paramMap map[string]interface{}, configName string) bool {
	provisioning, ok := paramMap["provisioning"].(mappings.ProvisioningConfig)
	return ok && provisioning.Installer.ExtraTemplate != "" && provisioning.Installer.ExtraTemplate != configName
}

func isStorageEncryptionEnabled(paramMap map[string]interface{}) bool {
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_enabled"])), "true")
}

func validateDebianPreseedParams(paramMap map[string]interface{}) error {
	mode := strings.TrimSpace(fmt.Sprint(paramMap["storage_mode"]))
	switch mode {
	case "regular", "lvm":
		return validateDebianEncryptionPreseedParams(paramMap)
	case "raid":
		if err := validateDebianRAIDPreseedParams(paramMap); err != nil {
			return err
		}
		return validateDebianEncryptionPreseedParams(paramMap)
	default:
		return errors.New(`preseed/debian storage_mode must be "regular", "lvm", or "raid", got ` + strconv.Quote(mode))
	}
}

func validateDebianEncryptionPreseedParams(paramMap map[string]interface{}) error {
	enabled := strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_enabled"]))
	if enabled != "true" && enabled != "false" {
		return errors.New(`preseed/debian storage_encryption_enabled must be "true" or "false", got ` + strconv.Quote(enabled))
	}
	paramMap["storage_encryption_enabled"] = enabled
	if err := validateDebianTPMPreseedParams(paramMap, enabled); err != nil {
		return err
	}
	if enabled == "false" {
		return nil
	}
	passphrase := ""
	if value, ok := paramMap["storage_encryption_passphrase"]; ok {
		passphrase = strings.TrimSpace(fmt.Sprint(value))
	}
	if passphrase == "" {
		return errors.New("preseed/debian storage_encryption_passphrase is required when storage_encryption_enabled is true")
	}
	if strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_cipher"])) == "" {
		return errors.New("preseed/debian storage_encryption_cipher must not be empty when storage_encryption_enabled is true")
	}
	if strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_hash"])) == "" {
		return errors.New("preseed/debian storage_encryption_hash must not be empty when storage_encryption_enabled is true")
	}
	keySize, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_key_size"])))
	if err != nil || keySize <= 0 {
		return errors.New("preseed/debian storage_encryption_key_size must be a positive integer when storage_encryption_enabled is true")
	}
	return nil
}

func validateDebianTPMPreseedParams(paramMap map[string]interface{}, encryptionEnabled string) error {
	enabled := strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_tpm_enabled"]))
	if enabled != "true" && enabled != "false" {
		return errors.New(`preseed/debian storage_encryption_tpm_enabled must be "true" or "false", got ` + strconv.Quote(enabled))
	}
	paramMap["storage_encryption_tpm_enabled"] = enabled
	if enabled == "false" {
		return nil
	}
	if encryptionEnabled != "true" {
		return errors.New("preseed/debian storage_encryption_tpm_enabled requires storage_encryption_enabled to be true")
	}
	if strings.TrimSpace(fmt.Sprint(paramMap["storage_mode"])) != "regular" {
		return errors.New("preseed/debian storage_encryption_tpm_enabled is supported only when storage_mode is regular")
	}
	if strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_tpm_initramfs"])) != "dracut" {
		return errors.New(`preseed/debian storage_encryption_tpm_initramfs must be "dracut" when TPM unlock is enabled`)
	}
	device := strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_tpm_device"]))
	if device == "" {
		return errors.New("preseed/debian storage_encryption_tpm_device must not be empty when TPM unlock is enabled")
	}
	if validation.ContainsShellUnsafeValue(device) {
		return errors.New("preseed/debian storage_encryption_tpm_device contains unsupported shell metacharacters")
	}
	paramMap["storage_encryption_tpm_device"] = device
	pcrs := strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_tpm_pcrs"]))
	if pcrs == "" {
		return errors.New("preseed/debian storage_encryption_tpm_pcrs must not be empty when TPM unlock is enabled")
	}
	if validation.ContainsShellUnsafeValue(pcrs) {
		return errors.New("preseed/debian storage_encryption_tpm_pcrs contains unsupported shell metacharacters")
	}
	if !validation.IsTPMPCRSelection(pcrs) {
		return errors.New("preseed/debian storage_encryption_tpm_pcrs must contain PCR numbers separated by + or comma")
	}
	paramMap["storage_encryption_tpm_pcrs"] = pcrs
	requireSHA256Bank := strings.TrimSpace(fmt.Sprint(paramMap["storage_encryption_tpm_require_sha256_bank"]))
	if requireSHA256Bank != "true" && requireSHA256Bank != "false" {
		return errors.New(`preseed/debian storage_encryption_tpm_require_sha256_bank must be "true" or "false", got ` + strconv.Quote(requireSHA256Bank))
	}
	paramMap["storage_encryption_tpm_require_sha256_bank"] = requireSHA256Bank
	return nil
}

func validateDebianRAIDPreseedParams(paramMap map[string]interface{}) error {
	level := strings.TrimSpace(fmt.Sprint(paramMap["storage_raid_level"]))
	if level != "1" {
		return errors.New(`preseed/debian storage_raid_level must be "1" for storage_mode "raid", got ` + strconv.Quote(level))
	}
	devicesRaw := ""
	if value, ok := paramMap["storage_raid_devices"]; ok {
		devicesRaw = fmt.Sprint(value)
	}
	devices := strings.Fields(devicesRaw)
	if len(devices) != 2 {
		return fmt.Errorf("preseed/debian storage_raid_devices must contain exactly 2 devices for storage_mode %q, got %d", "raid", len(devices))
	}
	for i, device := range devices {
		if !strings.HasPrefix(device, "/dev/") {
			return fmt.Errorf("preseed/debian storage_raid_devices[%d] must start with /dev/", i)
		}
		if strings.ContainsAny(device, "*?[") {
			return fmt.Errorf("preseed/debian storage_raid_devices[%d] must be an explicit /dev path without glob patterns", i)
		}
	}
	if _, ok := paramMap["storage_raid_device_0"]; !ok {
		paramMap["storage_raid_device_0"] = devices[0]
	}
	if _, ok := paramMap["storage_raid_device_1"]; !ok {
		paramMap["storage_raid_device_1"] = devices[1]
	}
	bootDegraded := strings.TrimSpace(fmt.Sprint(paramMap["storage_raid_boot_degraded"]))
	if bootDegraded != "true" && bootDegraded != "false" {
		return errors.New(`preseed/debian storage_raid_boot_degraded must be "true" or "false", got ` + strconv.Quote(bootDegraded))
	}
	return nil
}

func (s *ShoelacesTemplates) withInstallerExtra(paramMap map[string]interface{}, envName, configName string) (map[string]interface{}, error) {
	copied := make(map[string]interface{}, len(paramMap)+1)
	for key, value := range paramMap {
		copied[key] = value
	}
	copied["installerExtra"] = ""
	provisioning, ok := copied["provisioning"].(mappings.ProvisioningConfig)
	if !ok || provisioning.Installer.ExtraTemplate == "" || provisioning.Installer.ExtraTemplate == configName {
		return copied, nil
	}

	var b bytes.Buffer
	err := s.envTemplates[envName].templateObj.ExecuteTemplate(&b, provisioning.Installer.ExtraTemplate, copied)
	if err != nil && envName != defaultEnvironment {
		b.Reset()
		err = s.envTemplates[defaultEnvironment].templateObj.ExecuteTemplate(&b, provisioning.Installer.ExtraTemplate, copied)
	}
	if err != nil {
		return nil, err
	}
	copied["installerExtra"] = b.String()
	return copied, nil
}

func (s *ShoelacesTemplates) isEmbeddedTemplate(templateName, envName string) bool {
	if envName == "" {
		envName = defaultEnvironment
	}
	if e, ok := s.envTemplates[envName]; ok {
		return e.templateEmbedded[templateName]
	}
	return false
}

// ListVariables receives a template name and return the list of variables
// that belong to it. It's mainly used by the web frontend to provide a
// list of dynamic fields to complete before rendering a template.
func (s *ShoelacesTemplates) ListVariables(templateName, envName string) []string {
	if envName == "" {
		envName = defaultEnvironment
	}
	if e, ok := s.envTemplates[envName]; ok {
		return e.listVariables(templateName)
	}
	var empty []string
	return empty
}

func (e shoelacesTemplateEnvironment) listVariables(templateName string) []string {
	return e.collectVariables(templateName, make(map[string]bool))
}

func (e shoelacesTemplateEnvironment) collectVariables(templateName string, visited map[string]bool) []string {
	if visited[templateName] {
		return nil
	}
	// Guard against cyclic template references while still allowing shared
	// partials to be reached from multiple top-level templates.
	visited[templateName] = true

	var variables []string
	for _, variable := range e.templateVars[templateName] {
		if !utils.StringInSlice(variable, variables) {
			variables = append(variables, variable)
		}
	}
	for _, ref := range e.templateRefs[templateName] {
		for _, variable := range e.collectVariables(ref, visited) {
			if !utils.StringInSlice(variable, variables) {
				variables = append(variables, variable)
			}
		}
	}
	return variables
}
