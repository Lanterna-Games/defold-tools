package mobdebug

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	tagOfAStructure = "lua/structure"
	tagOfAReference = "lua/ref"
	tagOfATable     = "lua/table"
	keyOfTheValue   = ":value"
	keyOfTheRefs    = ":refs"
	keyOfTheContent = ":content"
	keyOfTheString  = ":string"
	wordForNothing  = "nil"
)

type tagged struct {
	tag  string
	form any
}

type keyword string

type nothing struct{}

type pair struct {
	key   any
	value any
}

type reader struct {
	text   string
	offset int
}

var errEndOfText = errors.New("the value ends before it is whole")

func (r *reader) skipBlanks() {
	for r.offset < len(r.text) && strings.ContainsRune(" \t\r\n,", rune(r.text[r.offset])) {
		r.offset++
	}
}

func (r *reader) form() (any, error) {
	r.skipBlanks()
	if r.offset >= len(r.text) {
		return nil, errEndOfText
	}
	switch r.text[r.offset] {
	case '#':
		start := r.offset + 1
		for r.offset < len(r.text) && !strings.ContainsRune("\"{[ ", rune(r.text[r.offset])) {
			r.offset++
		}
		tag := r.text[start:r.offset]
		form, err := r.form()
		return tagged{tag: tag, form: form}, err
	case '"':
		return r.quoted()
	case '{':
		forms, err := r.until('}')
		if err != nil || len(forms)%2 != 0 {
			return nil, errors.Join(err, errEndOfText)
		}
		pairs := make([]pair, 0, len(forms)/2)
		for index := 0; index < len(forms); index += 2 {
			pairs = append(pairs, pair{key: forms[index], value: forms[index+1]})
		}
		return pairs, nil
	case '[':
		return r.until(']')
	}
	start := r.offset
	for r.offset < len(r.text) && !strings.ContainsRune(" \t\r\n,]}", rune(r.text[r.offset])) {
		r.offset++
	}
	word := r.text[start:r.offset]
	switch {
	case word == wordForNothing:
		return nothing{}, nil
	case word == "true" || word == "false":
		return word == "true", nil
	case strings.HasPrefix(word, ":"):
		return keyword(word), nil
	}
	if number, err := strconv.ParseFloat(word, 64); err == nil {
		return number, nil
	}
	return word, nil
}

func (r *reader) quoted() (string, error) {
	var text strings.Builder
	for r.offset++; r.offset < len(r.text); r.offset++ {
		switch character := r.text[r.offset]; {
		case character == '"':
			r.offset++
			return text.String(), nil
		case character == '\\' && r.offset+1 < len(r.text):
			r.offset++
			switch escaped := r.text[r.offset]; escaped {
			case 'n':
				text.WriteByte('\n')
			case 't':
				text.WriteByte('\t')
			case 'r':
				text.WriteByte('\r')
			default:
				text.WriteByte(escaped)
			}
		default:
			text.WriteByte(character)
		}
	}
	return "", errEndOfText
}

func (r *reader) until(closing byte) ([]any, error) {
	forms := []any{}
	for r.offset++; ; {
		r.skipBlanks()
		if r.offset >= len(r.text) {
			return nil, errEndOfText
		}
		if r.text[r.offset] == closing {
			r.offset++
			return forms, nil
		}
		form, err := r.form()
		if err != nil {
			return nil, err
		}
		forms = append(forms, form)
	}
}

type Value struct {
	Text     string
	Kind     string
	Children []Named
}

type Named struct {
	Name  string
	Value Value
}

type structure struct {
	tables map[string]tagged
}

func valueOf(pairs []pair, key keyword) any {
	for _, each := range pairs {
		if each.key == key {
			return each.value
		}
	}
	return nil
}

func keyAsText(key any) string {
	switch typed := key.(type) {
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case string:
		return typed
	case tagged:
		return fmt.Sprint(typed.form)
	}
	return fmt.Sprint(key)
}

func (s structure) entriesOf(reference string) (entries []pair, text string, isATable bool) {
	table, isKnown := s.tables[reference]
	fields, isAMap := table.form.([]pair)
	if !isKnown || !isAMap {
		return nil, "", false
	}
	content, _ := valueOf(fields, keyOfTheContent).([]any)
	for index := 0; index+1 < len(content); index += 2 {
		entries = append(entries, pair{key: content[index], value: content[index+1]})
	}
	text, _ = valueOf(fields, keyOfTheString).(string)
	return entries, text, true
}

func (s structure) value(form any, depth int) Value {
	switch typed := form.(type) {
	case nil, nothing:
		return Value{Text: wordForNothing, Kind: wordForNothing}
	case bool:
		return Value{Text: strconv.FormatBool(typed), Kind: "boolean"}
	case float64:
		return Value{Text: strconv.FormatFloat(typed, 'g', -1, 64), Kind: "number"}
	case string:
		return Value{Text: strconv.Quote(typed), Kind: "string"}
	case tagged:
		reference, _ := typed.form.(string)
		if typed.tag != tagOfAReference {
			return Value{Text: reference, Kind: strings.TrimPrefix(typed.tag, "lua/")}
		}
		entries, text, isATable := s.entriesOf(reference)
		value := Value{Text: text, Kind: "table"}
		if !isATable || depth <= 0 {
			return value
		}
		for _, entry := range entries {
			value.Children = append(value.Children, Named{Name: keyAsText(entry.key), Value: s.value(entry.value, depth-1)})
		}
		sort.SliceStable(value.Children, func(left, right int) bool { return value.Children[left].Name < value.Children[right].Name })
		return value
	}
	return Value{Text: fmt.Sprint(form), Kind: "unknown"}
}

func structureIn(text string) (found structure, root any, err error) {
	parsed, err := (&reader{text: text}).form()
	if err != nil {
		return structure{}, nil, err
	}
	whole, isTagged := parsed.(tagged)
	fields, isAMap := whole.form.([]pair)
	if !isTagged || whole.tag != tagOfAStructure || !isAMap {
		return structure{}, nil, errors.New("the debugger answered with something that is no value")
	}
	found = structure{tables: map[string]tagged{}}
	refs, _ := valueOf(fields, keyOfTheRefs).([]pair)
	for _, each := range refs {
		reference, isAReference := each.key.(tagged)
		table, isATable := each.value.(tagged)
		if name, isNamed := reference.form.(string); isAReference && isATable && isNamed && table.tag == tagOfATable {
			found.tables[name] = table
		}
	}
	return found, valueOf(fields, keyOfTheValue), nil
}

func (s structure) at(form any, key float64) (any, bool) {
	reference, isAReference := form.(tagged)
	name, _ := reference.form.(string)
	if !isAReference {
		return nil, false
	}
	entries, _, _ := s.entriesOf(name)
	for _, entry := range entries {
		if entry.key == key {
			return entry.value, true
		}
	}
	return nil, false
}

func (s structure) named(form any) []pair {
	reference, isAReference := form.(tagged)
	name, _ := reference.form.(string)
	if !isAReference {
		return nil
	}
	entries, _, _ := s.entriesOf(name)
	return entries
}
