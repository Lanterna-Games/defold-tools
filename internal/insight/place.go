package insight

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
)

type Role int

const (
	RoleOfNothing Role = iota
	RoleOfAnAddress
	RoleOfAProperty
	RoleOfAMessagePosted
	RoleOfAMessageHandled
	RoleOfANode
	RoleOfAnAnimation
	RoleOfAnAnimationOfANode
	RoleOfAnAction
	RoleOfAGroup
	RoleOfAField
)

const (
	hashFunction        = "hash"
	postFunction        = "msg.post"
	urlFunction         = "msg.url"
	nodeLookupFunction  = "gui.get_node"
	spritePlayFunction  = "sprite.play_flipbook"
	nodePlayFunction    = "gui.play_flipbook"
	messageHandler      = "on_message"
	inputHandler        = "on_input"
	playAnimation       = "play_animation"
	animationOfTheTable = "id"
	groupOfTheOther     = ".other_group"
	groupOfTheOwn       = ".own_group"
	groupOfTheMessage   = ".group"
	messageOfTheHandler = 2
	fieldAccess         = '.'
	methodAccess        = ':'
)

var wordsThatStartAnotherStatement = map[string]bool{
	"function": true, "then": true, "do": true, "else": true, "elseif": true, "end": true,
	"local": true, "return": true, "if": true, "while": true, "for": true, "repeat": true, "until": true,
}

type Place struct {
	Role            Role
	Text            string
	Typed           string
	Start           int
	End             int
	Target          string
	TargetIsWritten bool
	TargetConstant  string
}

type surroundings struct {
	compared      string
	callee        string
	argument      int
	firstArgument []lua.Token
	secondText    string
	second        []lua.Token
	fieldOfATable string
	isInATable    bool
	isInACall     bool
}

func lineUpTo(content []byte, offset int) (quote byte, quoteStart int, isInAComment bool) {
	lineStart := bytes.LastIndexByte(content[:offset], '\n') + 1
	for index := lineStart; index < offset; index++ {
		character := content[index]
		switch {
		case quote == 0 && (character == '"' || character == '\''):
			quote, quoteStart = character, index+1
		case quote == 0 && character == '-' && index+1 < len(content) && content[index+1] == '-':
			return 0, 0, true
		case quote != 0 && character == '\\':
			index++
		case quote != 0 && character == quote:
			quote = 0
		}
	}
	return quote, quoteStart, false
}

func literalAround(content []byte, offset int) (start, end int, isInside bool) {
	if offset < 0 || offset > len(content) {
		return 0, 0, false
	}
	quote, start, _ := lineUpTo(content, offset)
	if quote == 0 {
		return 0, 0, false
	}
	end = offset
	for end < len(content) && content[end] != quote && content[end] != '\n' && content[end] != '\r' {
		if content[end] == '\\' {
			end++
		}
		end++
	}
	return start, min(end, len(content)), true
}

func isPartOfAName(character byte) bool {
	return character == '_' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func fieldAround(content []byte, offset int) (object string, start, end int, isAField bool) {
	if offset < 0 || offset > len(content) {
		return "", 0, 0, false
	}
	if quote, _, isInAComment := lineUpTo(content, offset); quote != 0 || isInAComment {
		return "", 0, 0, false
	}
	start, end = offset, offset
	for start > 0 && isPartOfAName(content[start-1]) {
		start--
	}
	for end < len(content) && isPartOfAName(content[end]) {
		end++
	}
	if start == 0 || content[start-1] != fieldAccess {
		return "", 0, 0, false
	}
	objectEnd := start - 1
	objectStart := objectEnd
	for objectStart > 0 && isPartOfAName(content[objectStart-1]) {
		objectStart--
	}
	isItselfAField := objectStart > 0 && (content[objectStart-1] == fieldAccess || content[objectStart-1] == methodAccess)
	if objectStart == objectEnd || isItselfAField {
		return "", 0, 0, false
	}
	return string(content[objectStart:objectEnd]), start, end, true
}

func isSymbol(token lua.Token, text string) bool {
	return token.Kind == lua.TokenSymbol && token.Text == text
}

func qualifiedNameEndingAt(tokens []lua.Token, last int) string {
	if last < 0 || tokens[last].Kind != lua.TokenName {
		return ""
	}
	name, first := tokens[last].Text, last
	for first >= 2 && isSymbol(tokens[first-1], ".") && tokens[first-2].Kind == lua.TokenName {
		name = tokens[first-2].Text + "." + name
		first -= 2
	}
	return name
}

func surroundingsOf(tokens []lua.Token) surroundings {
	var around surroundings
	last := len(tokens) - 1
	isTheHashFunction := func(index int) bool {
		isAField := index >= 1 && (isSymbol(tokens[index-1], ".") || isSymbol(tokens[index-1], ":"))
		return index >= 0 && tokens[index].Kind == lua.TokenName && tokens[index].Text == hashFunction && !isAField
	}
	switch {
	case last >= 1 && isSymbol(tokens[last], "(") && isTheHashFunction(last-1):
		last -= 2
	case isTheHashFunction(last):
		last--
	}
	if last < 0 {
		return around
	}
	if isSymbol(tokens[last], "==") || isSymbol(tokens[last], "~=") {
		around.compared = qualifiedNameEndingAt(tokens, last-1)
		return around
	}
	if last >= 1 && isSymbol(tokens[last], "=") && tokens[last-1].Kind == lua.TokenName {
		around.fieldOfATable = tokens[last-1].Text
		last -= 2
	}
	depth, commas := 0, 0
	for index := last; index >= 0; index-- {
		token := tokens[index]
		switch {
		case token.Kind == lua.TokenKeyword && depth == 0 && wordsThatStartAnotherStatement[token.Text]:
			return around
		case token.Kind != lua.TokenSymbol:
		case token.Text == ")" || token.Text == "]" || token.Text == "}":
			depth++
		case token.Text == "," && depth == 0:
			commas++
		case (token.Text == "(" || token.Text == "[" || token.Text == "{") && depth > 0:
			depth--
		case token.Text == "{":
			around.isInATable, commas = true, 0
		case token.Text == "(":
			around.callee = qualifiedNameEndingAt(tokens, index-1)
			around.argument, around.isInACall = commas, around.callee != ""
			arguments := argumentsFrom(tokens[index+1 : last+1])
			if len(arguments) > 0 && commas > 0 {
				around.firstArgument = arguments[0]
			}
			if len(arguments) > 1 && commas > 1 {
				around.second = arguments[1]
			}
			if len(around.second) == 1 && around.second[0].Kind == lua.TokenString {
				around.secondText = around.second[0].Text
			}
			return around
		case token.Text == "[":
			return around
		}
	}
	return around
}

func argumentsFrom(tokens []lua.Token) [][]lua.Token {
	var arguments [][]lua.Token
	var current []lua.Token
	depth := 0
	for _, token := range tokens {
		switch {
		case token.Kind == lua.TokenSymbol && (token.Text == "(" || token.Text == "[" || token.Text == "{"):
			depth++
		case token.Kind == lua.TokenSymbol && (token.Text == ")" || token.Text == "]" || token.Text == "}"):
			depth--
		case isSymbol(token, ",") && depth == 0:
			arguments = append(arguments, current)
			current = nil
			continue
		}
		current = append(current, token)
	}
	return append(arguments, current)
}

func handlerAround(chunk *lua.Chunk, offset int) (name string, parameters []string) {
	for _, statement := range chunk.Body {
		function, isFunction := statement.(*lua.FunctionStatement)
		if !isFunction || function.Method != nil {
			continue
		}
		plain, isPlain := function.Name.(*lua.Identifier)
		span := function.Span()
		if !isPlain || offset < span.Start.Offset || offset > span.End.Offset {
			continue
		}
		for _, parameter := range function.Function.Parameters {
			parameters = append(parameters, parameter.Name)
		}
		return plain.Name, parameters
	}
	return "", nil
}

func fieldOfAMessageAt(content []byte, offset int) (place Place, isSomewhere bool) {
	object, start, end, isAField := fieldAround(content, offset)
	if !isAField {
		return Place{}, false
	}
	chunk, _ := lua.Parse(content)
	handler, parameters := handlerAround(chunk, offset)
	if handler != messageHandler || len(parameters) <= messageOfTheHandler || parameters[messageOfTheHandler] != object {
		return Place{}, false
	}
	return Place{Role: RoleOfAField, Text: string(content[start:end]), Typed: string(content[start:offset]), Start: start, End: end}, true
}

func Locate(content []byte, offset int, reference *engine.Reference) (place Place, isSomewhere bool) {
	start, end, isInside := literalAround(content, offset)
	if !isInside {
		return fieldOfAMessageAt(content, offset)
	}
	place = Place{Text: string(content[start:end]), Typed: string(content[start:offset]), Start: start, End: end}
	around := surroundingsOf(lua.Tokens(content[:start-1]))
	chunk, _ := lua.Parse(content)
	handler, parameters := handlerAround(chunk, offset)
	place.Role = roleOf(around, handler, parameters, reference)
	if place.Role == RoleOfNothing {
		return Place{}, false
	}
	around.targetInto(&place)
	return place, true
}

func (s surroundings) targetInto(place *Place) {
	if len(s.firstArgument) != 1 {
		return
	}
	first := s.firstArgument[0]
	if first.Kind == lua.TokenString {
		place.Target, place.TargetIsWritten = first.Text, true
	}
	if first.Kind == lua.TokenName {
		place.TargetConstant = first.Text
	}
}

func roleOf(around surroundings, handler string, parameters []string, reference *engine.Reference) Role {
	parameter := func(index int) string {
		if index < len(parameters) {
			return parameters[index]
		}
		return "\x00"
	}
	switch {
	case around.compared != "" && handler == messageHandler && around.compared == parameter(1):
		return RoleOfAMessageHandled
	case around.compared != "" && handler == inputHandler && around.compared == parameter(1):
		return RoleOfAnAction
	case strings.HasSuffix(around.compared, groupOfTheOther), strings.HasSuffix(around.compared, groupOfTheOwn):
		return RoleOfAGroup
	case around.compared != "" && handler == messageHandler && around.compared == parameter(2)+groupOfTheMessage:
		return RoleOfAGroup
	case around.compared != "" || !around.isInACall:
		return RoleOfNothing
	}
	ofOneGroup, ofManyGroups := reference.CollisionGroupParameterIndexes(around.callee)
	if around.isInATable {
		switch {
		case around.fieldOfATable == "" && slices.Contains(ofManyGroups, around.argument):
			return RoleOfAGroup
		case around.fieldOfATable == animationOfTheTable && around.callee == postFunction && around.argument == 2 && around.secondText == playAnimation:
			return RoleOfAnAnimation
		default:
			return RoleOfNothing
		}
	}
	_, property, takesAProperty := reference.PropertyParameterIndexes(around.callee)
	switch {
	case takesAProperty && around.argument == property:
		return RoleOfAProperty
	case around.callee == postFunction && around.argument == 1:
		return RoleOfAMessagePosted
	case around.callee == nodeLookupFunction && around.argument == 0:
		return RoleOfANode
	case around.callee == spritePlayFunction && around.argument == 1:
		return RoleOfAnAnimation
	case around.callee == nodePlayFunction && around.argument == 1:
		return RoleOfAnAnimationOfANode
	case slices.Contains(ofOneGroup, around.argument):
		return RoleOfAGroup
	case slices.Contains(reference.URLParameterIndexes(around.callee), around.argument):
		return RoleOfAnAddress
	case around.callee == urlFunction && around.argument == 0:
		return RoleOfAnAddress
	default:
		return RoleOfNothing
	}
}
