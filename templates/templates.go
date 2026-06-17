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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"

	shoelaces "github.com/inngest/shoelaces"
	"github.com/inngest/shoelaces/log"
	"github.com/inngest/shoelaces/utils"
)

const defaultEnvironment = "default"

// ShoelacesTemplates holds the core attributes for handling the dyanmic configurations
// in Shoelaces.
type ShoelacesTemplates struct {
	envTemplates map[string]shoelacesTemplateEnvironment
	dataDir      string
	envDir       string
	tplExt       string
}

type shoelacesTemplateEnvironment struct {
	templateObj  *template.Template
	templateVars map[string][]string
	templateRefs map[string][]string
}

// New creates and initializes a new ShoelacesTemplates instance a returns a pointer to
// it.
func New() *ShoelacesTemplates {
	e := make(map[string]shoelacesTemplateEnvironment)
	e[defaultEnvironment] = shoelacesTemplateEnvironment{
		templateObj:  template.New(""),
		templateVars: make(map[string][]string),
		templateRefs: make(map[string][]string),
	}
	return &ShoelacesTemplates{envTemplates: e}
}

func (s *ShoelacesTemplates) checkAddEnvironment(logger log.Logger, environment string) {
	if _, ok := s.envTemplates[environment]; !ok {
		c, e := s.envTemplates[defaultEnvironment].templateObj.Clone()
		if e != nil {
			logger.Error("component", "template", "msg", "Template for environment already executed", "environment", environment)
			os.Exit(1)
		}
		s.envTemplates[environment] = shoelacesTemplateEnvironment{
			templateObj: c,
			// Environment overrides inherit the default index, then replace entries
			// for any templates they override.
			templateVars: cloneTemplateIndex(s.envTemplates[defaultEnvironment].templateVars),
			templateRefs: cloneTemplateIndex(s.envTemplates[defaultEnvironment].templateRefs),
		}
	}
}

func (s *ShoelacesTemplates) addTemplate(logger log.Logger, path string, environment string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return s.addTemplateContent(logger, path, content, environment)
}

func (s *ShoelacesTemplates) addTemplateContent(logger log.Logger, source string, content []byte, environment string) error {
	s.checkAddEnvironment(logger, environment)
	sourceName := filepath.Base(source)
	parsed, err := template.New(sourceName).Parse(string(content))
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
	}
	return nil
}

func cloneTemplateIndex(index map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(index))
	for name, values := range index {
		cloned[name] = append([]string(nil), values...)
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
	if value.Kind() == reflect.Ptr && value.IsNil() {
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
func (s *ShoelacesTemplates) ParseTemplates(logger log.Logger, dataDir string, envDir string, envs []string, tplExt string) {
	s.dataDir = dataDir
	s.envDir = envDir
	s.tplExt = tplExt

	logger.Debug("component", "template", "msg", "Template parsing started", "dir", dataDir)
	s.parseEmbeddedProvisioningTemplates(logger)

	tplScannerDefault := func(p string, info os.FileInfo, err error) error {
		if strings.HasPrefix(p, path.Join(dataDir, envDir)) {
			return err
		}
		if strings.HasSuffix(p, tplExt) {
			logger.Info("component", "template", "msg", "Parsing file", "file", p)
			if err := s.addTemplate(logger, p, defaultEnvironment); err != nil {
				logger.Error("component", "template", "err", err.Error())
				os.Exit(1)
			}
		}
		return err
	}

	tplScannerOverride := func(p string, info os.FileInfo, err error) error {
		if strings.HasSuffix(p, tplExt) {
			env := s.getEnvFromPath(p)
			logger.Info("component", "template", "msg", "Parsing ovveride", "environment", env, "file", p)

			if err := s.addTemplate(logger, p, env); err != nil {
				logger.Error("component", "template", "err", err.Error())
				os.Exit(1)
			}
		}
		return err
	}

	if err := filepath.Walk(dataDir, tplScannerDefault); err != nil {
		panic(err)
	}
	logger.Info("component", "template", "msg", "Parsing override files", "dir", path.Join(dataDir, envDir))
	if err := filepath.Walk(path.Join(dataDir, envDir), tplScannerOverride); err != nil {
		logger.Info("component", "template", "msg", "No overrides found")
	}
	logger.Debug("component", "template", "msg", "Parsing ended")
}

func (s *ShoelacesTemplates) parseEmbeddedProvisioningTemplates(logger log.Logger) {
	defaults := shoelaces.ProvisioningDefaultsFS()
	err := fs.WalkDir(defaults, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(p, s.tplExt) {
			logger.Info("component", "template", "msg", "Parsing embedded provisioning file", "file", p)
			content, err := fs.ReadFile(defaults, p)
			if err != nil {
				return err
			}
			return s.addTemplateContent(logger, p, content, defaultEnvironment)
		}
		return nil
	})
	if err != nil {
		logger.Error("component", "template", "err", err.Error())
		os.Exit(1)
	}
}

// RenderTemplate receives a name and a map of parameters, among other
// arguments, and returns the rendered template. It's aware of the
// environment, in case of any.
func (s *ShoelacesTemplates) RenderTemplate(logger log.Logger, configName string, paramMap map[string]interface{}, envName string) (string, error) {
	if envName == "" {
		envName = defaultEnvironment
	}
	logger.Info("component", "template", "action", "template-request", "template", configName, "env", envName, "parameters", utils.RedactedMapToString(paramMap))

	requiredVariables := s.envTemplates[envName].listVariables(configName)

	var b bytes.Buffer
	err := s.envTemplates[envName].templateObj.ExecuteTemplate(&b, configName, paramMap)
	// Fall back to default template in case this is non default environment
	// XXX: this is temporary and will be simplified to reduce the code duplication
	if err != nil && envName != defaultEnvironment {
		requiredVariables = s.envTemplates[defaultEnvironment].listVariables(configName)
		err = s.envTemplates[defaultEnvironment].templateObj.ExecuteTemplate(&b, configName, paramMap)
	}
	if err != nil {
		logger.Info("component", "template", "action", "render-template", "err", err.Error())
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
		logger.Info("component", "template", "msg", "Missing variables in request", "variables", missingVariables)
		return "", errors.New("Missing variables in request: " + missingVariables)
	}

	return r, nil
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
