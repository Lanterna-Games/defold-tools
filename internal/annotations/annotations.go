package annotations

import (
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

const (
	fileExtension      = ".lua"
	typesFile          = "defold" + fileExtension
	namespaceOfGlobals = "builtins"
	header             = `---@meta
---@diagnostic disable: lowercase-global
---@diagnostic disable: missing-return
---@diagnostic disable: args-after-dots
---@diagnostic disable: duplicate-doc-alias
---@diagnostic disable: duplicate-doc-field
---@diagnostic disable: duplicate-set-field
---@diagnostic disable: undefined-doc-name

`
)

var namespacesTheLanguageServerKnows = map[string]bool{
	"base": true, "coroutine": true, "debug": true, "io": true, "math": true,
	"os": true, "package": true, "string": true, "table": true,
}

var namespacesOfTheEditor = regexp.MustCompile(`^editor(\.|$)`)

func Globals(reference *docs.Reference) []string {
	writer := generationOf(reference, docs.Links{})
	isGlobal := map[string]bool{}
	for _, module := range reference.Modules {
		if !IsWritten(module.Namespace) {
			continue
		}
		if module.Namespace == namespaceOfGlobals {
			for _, function := range module.Functions {
				if _, isWritten := writer.function(module, function, true); isWritten {
					isGlobal[function.Name] = true
				}
			}
			continue
		}
		if _, hasAnything := writer.module(module); hasAnything {
			outermost, _, _ := strings.Cut(module.Namespace, ".")
			isGlobal[outermost] = true
		}
	}
	return slices.Sorted(maps.Keys(isGlobal))
}

func generationOf(reference *docs.Reference, links docs.Links) *generation {
	writer := &generation{version: reference.Version, links: links, isDefined: map[string]bool{}, isNamespace: map[string]bool{}}
	for _, module := range reference.Modules {
		if IsWritten(module.Namespace) {
			writer.isNamespace[module.Namespace] = true
		}
	}
	return writer
}

func IsWritten(namespace string) bool {
	return !namespacesTheLanguageServerKnows[namespace] && !namespacesOfTheEditor.MatchString(namespace)
}

func Generate(reference *docs.Reference, links docs.Links) map[string]string {
	files := map[string]string{}
	writer := generationOf(reference, links)
	for _, module := range reference.Modules {
		if !IsWritten(module.Namespace) {
			continue
		}
		if content, hasAnything := writer.module(module); hasAnything {
			files[strings.ReplaceAll(module.Namespace, ".", "_")+fileExtension] = content
		}
	}
	files[typesFile] = writer.types()
	return files
}

type generation struct {
	version     string
	links       docs.Links
	isDefined   map[string]bool
	isNamespace map[string]bool
	mentioned   []string
}

func commented(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	var written strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		written.WriteString("---" + line + "\n")
	}
	return written.String()
}

func (g *generation) module(module docs.Module) (content string, hasAnything bool) {
	var written strings.Builder
	written.WriteString(header)

	isOfGlobals := module.Namespace == namespaceOfGlobals
	fields := g.constantsOf(module)
	if !isOfGlobals {
		g.defineParentsOf(module.Namespace, &written)
		written.WriteString(commented(docs.Markdown(firstOf(module.Description, module.Brief))))
		written.WriteString("---@class defold." + module.Namespace + "\n" + fields)
		written.WriteString(module.Namespace + " = {}\n\n")
		g.isDefined[module.Namespace] = true
		hasAnything = fields != ""
	}
	for _, described := range module.Types {
		written.WriteString(g.typeOf(described))
		hasAnything = true
	}
	for _, function := range module.Functions {
		if declared, isWritten := g.function(module, function, isOfGlobals); isWritten {
			written.WriteString(declared)
			hasAnything = true
		}
	}
	return written.String(), hasAnything
}

func firstOf(texts ...string) string {
	for _, text := range texts {
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func (g *generation) defineParentsOf(namespace string, written *strings.Builder) {
	parts := strings.Split(namespace, ".")
	for depth := 1; depth < len(parts); depth++ {
		parent := strings.Join(parts[:depth], ".")
		if !g.isDefined[parent] && !g.isNamespace[parent] {
			written.WriteString(parent + " = {}\n\n")
			g.isDefined[parent] = true
		}
	}
}

func (g *generation) constantsOf(module docs.Module) string {
	var written strings.Builder
	seen := map[string]bool{}
	for _, constant := range module.Constants {
		name, isOfTheModule := strings.CutPrefix(constant.Name, module.Namespace+".")
		if !isOfTheModule || !isAnIdentifier(name) || seen[name] {
			continue
		}
		seen[name] = true
		written.WriteString(commented(docs.Markdown(constant.Doc)))
		written.WriteString(commented("\n" + g.links.To(g.version, module.Namespace, constant.Name, firstOf(constant.Type, constant.Name))))
		written.WriteString("---@field " + name + " " + g.typeNamed(firstOf(constant.Type, "constant")) + "\n")
	}
	return written.String()
}

func (g *generation) typeOf(described docs.Type) string {
	if !isATypeName(described.Name) {
		return ""
	}
	var written strings.Builder
	written.WriteString(commented(docs.Markdown(described.Doc)))
	g.isDefined[described.Name] = true
	switch {
	case described.Kind == docs.KindOfAlias && len(described.Fields) > 0:
		var alternatives []string
		for _, alternative := range described.Fields {
			alternatives = append(alternatives, g.typesNamed(alternative.Types))
		}
		written.WriteString("---@alias " + described.Name + " " + strings.Join(alternatives, "|") + "\n\n")
	default:
		written.WriteString("---@class " + described.Name + "\n")
		for _, field := range described.Fields {
			if !isAnIdentifier(field.Name) {
				continue
			}
			optional := ""
			if field.IsOptional {
				optional = "?"
			}
			written.WriteString("---@field " + field.Name + optional + " " + g.typesNamed(field.Types) + " " + docs.OneLine(field.Doc) + "\n")
		}
		written.WriteString("\n")
	}
	return written.String()
}

func (g *generation) function(module docs.Module, function docs.Function, isOfGlobals bool) (declared string, isWritten bool) {
	isOfTheModule := strings.HasPrefix(function.Name, module.Namespace+".") && isAQualifiedName(function.Name)
	isAGlobal := isOfGlobals && isAnIdentifier(function.Name)
	if !isOfTheModule && !isAGlobal {
		return "", false
	}
	var written strings.Builder
	g.defineParentsOf(function.Name, &written)
	written.WriteString(commented(docs.Markdown(firstOf(function.Description, function.Brief))))
	if examples := docs.Markdown(function.Examples); examples != "" {
		written.WriteString(commented("\n**Examples:**\n\n" + examples))
	}
	names := parameterNames(function.Parameters)
	for index, parameter := range function.Parameters {
		optional := ""
		if parameter.IsOptional && names[index] != "..." {
			optional = "?"
		}
		written.WriteString(strings.TrimRight("---@param "+names[index]+optional+" "+g.typesNamed(parameter.Types)+" "+docs.OneLine(parameter.Doc), " ") + "\n")
	}
	for _, returned := range function.Returns {
		written.WriteString(returnLine(g.typesNamed(returned.Types), identifierFrom(returned.Name), docs.OneLine(returned.Doc)))
	}
	written.WriteString(commented("\n" + g.links.To(g.version, module.Namespace, function.Name, anchorOnTheWebOf(function))))
	written.WriteString("function " + function.Name + "(" + strings.Join(names, ", ") + ") end\n\n")
	return written.String(), true
}

func returnLine(types, name, doc string) string {
	switch {
	case name != "" && doc != "":
		return "---@return " + types + " " + name + " " + doc + "\n"
	case name != "":
		return "---@return " + types + " " + name + "\n"
	case doc != "":
		return "---@return " + types + " # " + doc + "\n"
	default:
		return "---@return " + types + "\n"
	}
}

func anchorOnTheWebOf(function docs.Function) string {
	names := make([]string, 0, len(function.Parameters))
	for _, parameter := range function.Parameters {
		names = append(names, parameter.Name)
	}
	if len(names) == 0 {
		return function.Name
	}
	return function.Name + ":" + strings.Join(names, "-")
}

func (g *generation) typesNamed(written []string) string {
	if len(written) == 0 {
		return "any"
	}
	named := make([]string, 0, len(written))
	for _, one := range written {
		named = append(named, g.typeNamed(one))
	}
	return strings.Join(slices.Compact(named), "|")
}

func (g *generation) typeNamed(written string) string {
	named := normalised(written)
	g.mentioned = append(g.mentioned, namesMentionedIn(named)...)
	return named
}

func (g *generation) types() string {
	var written strings.Builder
	written.WriteString(header)
	written.WriteString(typesOfTheEngine)
	for _, defined := range typesOfTheEngineByName {
		g.isDefined[defined] = true
	}
	sort.Strings(g.mentioned)
	for _, name := range slices.Compact(g.mentioned) {
		if !g.isDefined[name] && !typesOfTheLanguage[name] {
			written.WriteString("---@class " + name + "\n\n")
		}
	}
	return written.String()
}
