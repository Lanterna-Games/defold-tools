package annotations

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

const typesOfTheEngine = `---@class hash

---@class url
---@field socket hash|string
---@field path hash|string
---@field fragment hash|string

---@class vector3
---@field x number
---@field y number
---@field z number
---@operator add(vector3): vector3
---@operator sub(vector3): vector3
---@operator mul(number): vector3
---@operator div(number): vector3
---@operator unm: vector3

---@class vector4
---@field x number
---@field y number
---@field z number
---@field w number
---@operator add(vector4): vector4
---@operator sub(vector4): vector4
---@operator mul(number): vector4
---@operator div(number): vector4
---@operator unm: vector4

---@class quaternion
---@field x number
---@field y number
---@field z number
---@field w number
---@operator mul(quaternion): quaternion

---@class matrix4
---@field c0 vector4
---@field c1 vector4
---@field c2 vector4
---@field c3 vector4
---@operator mul(matrix4): matrix4
---@operator mul(vector4): vector4

---@class vector

---@class node

---@class constant

---@class script_instance

`

const scopeOfAnotherLanguage = "::"

var typesOfTheEngineByName = []string{
	"hash", "url", "vector3", "vector4", "quaternion", "matrix4", "vector", "node", "constant", "script_instance",
}

var typesOfTheLanguage = map[string]bool{
	"nil": true, "any": true, "boolean": true, "string": true, "number": true, "integer": true,
	"function": true, "table": true, "thread": true, "userdata": true, "lightuserdata": true,
	"fun": true, "self": true, "true": true, "false": true, "unknown": true,
}

var wordsOfTheLanguage = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true, "false": true,
	"for": true, "function": true, "if": true, "in": true, "local": true, "nil": true, "not": true,
	"or": true, "repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
	"goto": true,
}

var typesUnderOtherNames = map[string]string{
	"bool": "boolean", "int": "integer", "uint": "integer", "float": "number", "double": "number",
	"numbers": "number", "str": "string", "object": "any", "userdata": "userdata",
	"vmath.vector3": "vector3", "vmath.vector4": "vector4", "vmath.quaternion": "quaternion",
	"vmath.matrix4": "matrix4", "vmath.vector": "vector",
}

var (
	identifier           = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	qualifiedName        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	notOfAnIdentifier    = regexp.MustCompile(`[^A-Za-z0-9_]+`)
	functionAsTheDocsSay = regexp.MustCompile(`^function\s*\((.*)\)$`)
	nameInAType          = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*`)
	charactersOfAType    = regexp.MustCompile(`^[A-Za-z0-9_.,:|?<>(){}\[\] "'\-]+$`)
)

func isAnIdentifier(name string) bool {
	return identifier.MatchString(name) && !wordsOfTheLanguage[name]
}

func isAQualifiedName(name string) bool {
	if !qualifiedName.MatchString(name) {
		return false
	}
	for part := range strings.SplitSeq(name, ".") {
		if wordsOfTheLanguage[part] {
			return false
		}
	}
	return true
}

func isATypeName(name string) bool {
	return qualifiedName.MatchString(name)
}

func identifierFrom(name string) string {
	cleaned := strings.Trim(notOfAnIdentifier.ReplaceAllString(name, "_"), "_")
	switch {
	case cleaned == "":
		return ""
	case cleaned[0] >= '0' && cleaned[0] <= '9', wordsOfTheLanguage[cleaned]:
		return "_" + cleaned
	default:
		return cleaned
	}
}

func parameterNames(parameters []docs.Value) []string {
	names := make([]string, 0, len(parameters))
	timesUsed := map[string]int{}
	for index, parameter := range parameters {
		name := identifierFrom(parameter.Name)
		isTheRest := strings.Contains(parameter.Name, "...")
		switch {
		case isTheRest && index == len(parameters)-1:
			names = append(names, "...")
			continue
		case name == "":
			name = "arg" + strconv.Itoa(index+1)
		}
		timesUsed[name]++
		if timesUsed[name] > 1 {
			name += strconv.Itoa(timesUsed[name])
		}
		names = append(names, name)
	}
	return names
}

func normalised(written string) string {
	written = strings.TrimSpace(written)
	if under, isOtherName := typesUnderOtherNames[written]; isOtherName {
		return under
	}
	if arguments := functionAsTheDocsSay.FindStringSubmatch(written); arguments != nil {
		return "fun(" + argumentsOfAFunction(arguments[1]) + ")"
	}
	isNotOfScripts := strings.Contains(written, scopeOfAnotherLanguage)
	if written == "" || isNotOfScripts || !charactersOfAType.MatchString(written) || !isBalanced(written) {
		return "any"
	}
	return written
}

func argumentsOfAFunction(written string) string {
	var arguments []string
	for argument := range strings.SplitSeq(written, ",") {
		if name := identifierFrom(argument); name != "" {
			arguments = append(arguments, name)
		}
	}
	return strings.Join(arguments, ", ")
}

func isBalanced(written string) bool {
	closes := map[rune]rune{')': '(', ']': '[', '}': '{', '>': '<'}
	var open []rune
	for _, character := range written {
		switch character {
		case '(', '[', '{', '<':
			open = append(open, character)
		case ')', ']', '}', '>':
			if len(open) == 0 || open[len(open)-1] != closes[character] {
				return false
			}
			open = open[:len(open)-1]
		}
	}
	return len(open) == 0
}

func namesMentionedIn(named string) []string {
	var names []string
	for _, found := range nameInAType.FindAllStringIndex(named, -1) {
		before := strings.TrimRight(named[:found[0]], " ")
		after := strings.TrimLeft(named[found[1]:], " ")
		isTheNameOfAField := strings.HasPrefix(after, ":") || strings.HasPrefix(after, "?:")
		isAnArgumentWithoutAType := (strings.HasSuffix(before, "(") || strings.HasSuffix(before, ",")) &&
			(strings.HasPrefix(after, ")") || strings.HasPrefix(after, ","))
		if name := named[found[0]:found[1]]; !isTheNameOfAField && !isAnArgumentWithoutAType && !typesOfTheLanguage[name] {
			names = append(names, name)
		}
	}
	return names
}
