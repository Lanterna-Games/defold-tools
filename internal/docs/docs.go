package docs

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

const (
	documentSuffix       = "_doc.json"
	languageOfScripts    = "Lua"
	markOfAnOptionalName = "?"
	trueAsTheDocsSayIt   = "True"
)

var (
	namespaceOfScripts    = regexp.MustCompile(`^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)*$`)
	optionalInBrackets    = regexp.MustCompile(`^\[(.*)\]$`)
	nameOfAScriptFunction = regexp.MustCompile(`^[a-z_][a-zA-Z0-9_]*([.:][a-zA-Z_][a-zA-Z0-9_]*)*$`)
)

type Reference struct {
	Version string   `json:"version"`
	Modules []Module `json:"modules"`
}

type Module struct {
	Namespace   string     `json:"namespace"`
	Name        string     `json:"name"`
	Brief       string     `json:"brief,omitempty"`
	Description string     `json:"description,omitempty"`
	Functions   []Function `json:"functions,omitempty"`
	Constants   []Constant `json:"constants,omitempty"`
	Types       []Type     `json:"types,omitempty"`
	Messages    []Message  `json:"messages,omitempty"`
	Properties  []Property `json:"properties,omitempty"`
}

type Value struct {
	Name       string   `json:"name"`
	Types      []string `json:"types,omitempty"`
	Doc        string   `json:"doc,omitempty"`
	IsOptional bool     `json:"optional,omitempty"`
}

type Function struct {
	Name        string  `json:"name"`
	Brief       string  `json:"brief,omitempty"`
	Description string  `json:"description,omitempty"`
	Examples    string  `json:"examples,omitempty"`
	Parameters  []Value `json:"parameters,omitempty"`
	Returns     []Value `json:"returns,omitempty"`
}

type Constant struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Doc  string `json:"doc,omitempty"`
}

type Type struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Doc    string  `json:"doc,omitempty"`
	Fields []Value `json:"fields,omitempty"`
}

type Message struct {
	Name        string  `json:"name"`
	Brief       string  `json:"brief,omitempty"`
	Description string  `json:"description,omitempty"`
	Examples    string  `json:"examples,omitempty"`
	Fields      []Value `json:"fields,omitempty"`
}

type Property struct {
	Name        string `json:"name"`
	Brief       string `json:"brief,omitempty"`
	Description string `json:"description,omitempty"`
	Examples    string `json:"examples,omitempty"`
}

const (
	KindOfEnum   = "enum"
	KindOfStruct = "struct"
	KindOfAlias  = "alias"
)

type document struct {
	Info struct {
		Namespace   string `json:"namespace"`
		Name        string `json:"name"`
		Brief       string `json:"brief"`
		Description string `json:"description"`
		Language    string `json:"language"`
	} `json:"info"`
	Elements []element `json:"elements"`
}

type element struct {
	Type         string      `json:"type"`
	Name         string      `json:"name"`
	Brief        string      `json:"brief"`
	Description  string      `json:"description"`
	Examples     string      `json:"examples"`
	Parameters   []parameter `json:"parameters"`
	ReturnValues []parameter `json:"returnvalues"`
	Members      []member    `json:"members"`
}

type parameter struct {
	Name       string   `json:"name"`
	Doc        string   `json:"doc"`
	Types      []string `json:"types"`
	IsOptional string   `json:"is_optional"`
}

type member struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
	Type string `json:"type"`
}

func FromArchive(version string, archive []byte) (*Reference, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("opening the documentation of Defold %s: %w", version, err)
	}
	byNamespace := map[string]*Module{}
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, documentSuffix) {
			continue
		}
		parsed, err := read(file)
		if err != nil {
			return nil, err
		}
		if !isOfScripts(parsed) {
			continue
		}
		module, isKnown := byNamespace[parsed.Info.Namespace]
		if !isKnown {
			module = &Module{Namespace: parsed.Info.Namespace}
			byNamespace[parsed.Info.Namespace] = module
		}
		module.add(parsed)
	}
	reference := &Reference{Version: version}
	for _, namespace := range sortedKeys(byNamespace) {
		if module := byNamespace[namespace]; module.documentsSomething() {
			reference.Modules = append(reference.Modules, *module)
		}
	}
	if len(reference.Modules) == 0 {
		return nil, fmt.Errorf("the documentation of Defold %s holds nothing for scripts: it is not written the way this build reads", version)
	}
	return reference, nil
}

func read(file *zip.File) (*document, error) {
	opened, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", file.Name, err)
	}
	defer opened.Close()
	content, err := io.ReadAll(opened)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file.Name, err)
	}
	var parsed document
	if err := json.Unmarshal(content, &parsed); err != nil {
		return nil, fmt.Errorf("reading %s: %w", file.Name, err)
	}
	return &parsed, nil
}

func isOfScripts(parsed *document) bool {
	if parsed.Info.Language != "" {
		return parsed.Info.Language == languageOfScripts
	}
	if !namespaceOfScripts.MatchString(parsed.Info.Namespace) {
		return false
	}
	for _, described := range parsed.Elements {
		if described.Type == "FUNCTION" && !nameOfAScriptFunction.MatchString(described.Name) {
			return false
		}
	}
	return true
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func (m *Module) documentsSomething() bool {
	return len(m.Functions)+len(m.Constants)+len(m.Types)+len(m.Messages)+len(m.Properties) > 0
}

func (m *Module) add(parsed *document) {
	if m.Name == "" {
		m.Name, m.Brief, m.Description = parsed.Info.Name, parsed.Info.Brief, parsed.Info.Description
	}
	for _, described := range parsed.Elements {
		switch described.Type {
		case "FUNCTION":
			m.Functions = append(m.Functions, Function{
				Name:        described.Name,
				Brief:       described.Brief,
				Description: described.Description,
				Examples:    described.Examples,
				Parameters:  valuesFrom(described.Parameters),
				Returns:     valuesFrom(described.ReturnValues),
			})
		case "VARIABLE", "CONSTANT":
			m.Constants = append(m.Constants, Constant{Name: described.Name, Doc: firstOf(described.Brief, described.Description)})
		case "ENUM":
			m.Types = append(m.Types, Type{Name: described.Name, Kind: KindOfEnum, Doc: firstOf(described.Brief, described.Description)})
			for _, value := range described.Members {
				m.Constants = append(m.Constants, Constant{Name: value.Name, Type: described.Name, Doc: value.Doc})
			}
		case "STRUCT", "CLASS":
			m.Types = append(m.Types, Type{Name: described.Name, Kind: KindOfStruct, Doc: firstOf(described.Brief, described.Description), Fields: fieldsFrom(described.Members)})
		case "TYPEDEF":
			aliased := Type{Name: described.Name, Kind: KindOfAlias, Doc: firstOf(described.Brief, described.Description)}
			for _, value := range described.Members {
				aliased.Fields = append(aliased.Fields, Value{Name: value.Name, Types: typesIn(value.Type), Doc: value.Doc})
			}
			m.Types = append(m.Types, aliased)
		case "MESSAGE":
			m.Messages = append(m.Messages, Message{
				Name:        described.Name,
				Brief:       described.Brief,
				Description: described.Description,
				Examples:    described.Examples,
				Fields:      valuesFrom(described.Parameters),
			})
		case "PROPERTY":
			m.Properties = append(m.Properties, Property{
				Name:        described.Name,
				Brief:       described.Brief,
				Description: described.Description,
				Examples:    described.Examples,
			})
		}
	}
}

func firstOf(texts ...string) string {
	for _, text := range texts {
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func valuesFrom(parameters []parameter) []Value {
	values := make([]Value, 0, len(parameters))
	for _, described := range parameters {
		value := Value{Name: described.Name, Doc: described.Doc, IsOptional: described.IsOptional == trueAsTheDocsSayIt}
		if inBrackets := optionalInBrackets.FindStringSubmatch(described.Name); inBrackets != nil {
			value.Name, value.IsOptional = inBrackets[1], true
		}
		for _, written := range described.Types {
			value.Types = append(value.Types, typesIn(written)...)
		}
		values = append(values, value)
	}
	return values
}

func fieldsFrom(members []member) []Value {
	fields := make([]Value, 0, len(members))
	for _, described := range members {
		name, isOptional := strings.CutSuffix(described.Name, markOfAnOptionalName)
		fields = append(fields, Value{Name: name, Types: typesIn(described.Type), Doc: described.Doc, IsOptional: isOptional})
	}
	return fields
}

func typesIn(written string) []string {
	written = strings.TrimSpace(written)
	if written == "" {
		return nil
	}
	return []string{written}
}
