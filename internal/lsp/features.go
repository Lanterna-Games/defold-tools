package lsp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/insight"
	"github.com/Lanterna-Games/defold-tools/internal/project"
	"github.com/Lanterna-Games/defold-tools/internal/suggest"
)

const (
	markdownKind           = "markdown"
	methodThatLeadsToAFile = "textDocument/definition"
	quickFixKind           = "quickfix"
	titleOfRemovingALine   = "Remove the line"
	separatorsOfAnAddress  = "#/:"
	separatorOfAnElement   = "."
	startOfAnAbsolutePath  = "/"
	startOfAFragment       = '#'
	kindOfAReference       = 18
	kindOfAProperty        = 10
	kindOfAnEvent          = 23
	kindOfAField           = 5
	kindOfAValue           = 12
	kindOfAFile            = 17
	kindOfAnEnumMember     = 20
)

var charactersThatStartAName = []string{`"`, `'`, "#", "/", ":", "."}

var methodsAnsweredForData = map[string]bool{
	methodThatLeadsToAFile: true, "textDocument/completion": true, "textDocument/hover": true, "textDocument/codeAction": true, "textDocument/references": true,
}

var extensionsOfScripts = map[string]bool{".script": true, ".gui_script": true, ".render_script": true, ".lua": true}

var kindOfARole = map[insight.Role]int{
	insight.RoleOfAnAddress:          kindOfAReference,
	insight.RoleOfAProperty:          kindOfAProperty,
	insight.RoleOfAMessagePosted:     kindOfAnEvent,
	insight.RoleOfAMessageHandled:    kindOfAnEvent,
	insight.RoleOfANode:              kindOfAField,
	insight.RoleOfAnAnimation:        kindOfAValue,
	insight.RoleOfAnAnimationOfANode: kindOfAValue,
	insight.RoleOfAnAction:           kindOfAnEnumMember,
	insight.RoleOfAGroup:             kindOfAnEnumMember,
	insight.RoleOfAField:             kindOfAField,
}

type asked struct {
	root    string
	file    string
	uri     string
	content []byte
	offset  int
	within  textRange
	newName string

	isOfSettings bool
	isOfData     bool
	insight      insight.Insight
	problems     []check.Problem
}

func (s *Server) answerAbout(received *message, answer func(asked) any) error {
	if received.ID == nil {
		return nil
	}
	var params positionParams
	if err := json.Unmarshal(received.Params, &params); err != nil {
		return s.connection.refuse(received.ID, requestWasInvalid, err.Error())
	}
	about, isInTheProject := s.whatIsAsked(params)
	if !isInTheProject || (about.isOfData && !methodsAnsweredForData[received.Method]) {
		return s.connection.answer(received.ID, nil)
	}
	answered := answer(about)
	if refused, isRefused := answered.(refusal); isRefused {
		return s.connection.refuse(received.ID, requestFailed, refused.reason)
	}
	return s.connection.answer(received.ID, answered)
}

type refusal struct {
	reason string
}

func (s *Server) contentOf(root, file string) ([]byte, error) {
	s.state.Lock()
	unsaved, isOpen := s.unsaved[file]
	s.state.Unlock()
	if isOpen {
		return unsaved, nil
	}
	return os.ReadFile(filepath.FromSlash(root + "/" + file))
}

func (s *Server) references(about asked) any {
	if named, isAFile := about.insight.FileNamedAt(about.content, about.offset); isAFile {
		places := about.insight.PlacesThatName(named, func(file string) ([]byte, error) { return s.contentOf(about.root, file) })
		return s.placesOf(about.root, places)
	}
	if about.isOfData {
		return nil
	}
	found, name := about.insight.References(about.file, about.content, about.offset)
	locations := make([]location, 0, len(found))
	for _, where := range found {
		content, err := s.contentOf(about.root, where.File)
		if err != nil {
			continue
		}
		start := where.Position.Offset
		locations = append(locations, location{
			URI:   uriOf(about.root + "/" + where.File),
			Range: textRange{Start: positionAt(content, start), End: positionAt(content, start+len(name))},
		})
	}
	return locations
}

func (s *Server) whatCanBeRenamed(about asked) any {
	name, start, end, err := about.insight.Renamable(about.file, about.content, about.offset)
	if errors.Is(err, insight.ErrNothingToRename) {
		return nil
	}
	if err != nil {
		return refusal{reason: err.Error()}
	}
	return whatCanBeRenamed{
		Range:       textRange{Start: positionAt(about.content, start), End: positionAt(about.content, end)},
		Placeholder: name,
	}
}

func (s *Server) rename(about asked) any {
	renaming, err := about.insight.Rename(about.file, about.content, about.offset, about.newName)
	if err != nil {
		return refusal{reason: err.Error()}
	}
	edits := map[string][]textEdit{}
	for _, change := range renaming.Changes {
		content, err := s.contentOf(about.root, change.Where.File)
		if err != nil {
			return refusal{reason: fmt.Sprintf("%s could not be read, so nothing was renamed: %v", change.Where.File, err)}
		}
		if !change.Fits(content) {
			return refusal{reason: change.Where.File + " changed since the project was last read, so nothing was renamed. Try again"}
		}
		start := change.Where.Position.Offset
		uri := uriOf(about.root + "/" + change.Where.File)
		edits[uri] = append(edits[uri], textEdit{
			Range:   textRange{Start: positionAt(content, start), End: positionAt(content, start+len(change.Old))},
			NewText: change.New,
		})
	}
	if len(renaming.LeftAlone) > 0 {
		s.warn(whatARenameLeavesAlone(renaming))
	}
	return workspaceEdit{Changes: edits}
}

func whatARenameLeavesAlone(renaming insight.Renaming) string {
	places := make([]string, 0, len(renaming.LeftAlone))
	for _, where := range renaming.LeftAlone {
		places = append(places, fmt.Sprintf("%s:%d", where.File, where.Position.Line))
	}
	return fmt.Sprintf("Renaming %q leaves the same text alone in %s, because what it is used for there cannot be read. Look at it before going on.",
		renaming.Name, strings.Join(places, ", "))
}

func (s *Server) whatIsAsked(params positionParams) (about asked, isInTheProject bool) {
	relative, isInTheProject := s.relative(params.TextDocument.URI)
	s.state.Lock()
	analysed, root, documentation := s.analysed, s.root, s.documentation
	content, isOpen := s.unsaved[relative]
	s.state.Unlock()
	isOfSettings := relative == insight.SettingsFile
	if !isInTheProject || analysed.Project == nil {
		return asked{}, false
	}
	if !isOpen {
		var err error
		if content, err = os.ReadFile(filepath.FromSlash(root + "/" + relative)); err != nil {
			return asked{}, false
		}
	}
	return asked{
		root:    root,
		file:    relative,
		uri:     params.TextDocument.URI,
		content: content,
		offset:  offsetAt(content, params.Position),
		within:  params.Range,
		newName: params.NewName,

		isOfSettings: isOfSettings,
		isOfData:     !isOfSettings && !extensionsOfScripts[filepath.Ext(relative)],
		insight:      insight.Insight{Project: analysed.Project, Reference: analysed.Reference, Documentation: documentation},
		problems:     analysed.Problems,
	}, true
}

func (s *Server) completion(about asked) any {
	candidates, start, end := about.insight.Complete(about.file, about.content, about.offset)
	switch {
	case about.isOfSettings:
		candidates, start, end = about.insight.CompleteSetting(about.content, about.offset)
	case about.isOfData:
		candidates, start, end = about.insight.CompleteInData(about.file, about.content, about.offset)
	}
	items := make([]completionItem, 0, len(candidates))
	replaced := textRange{Start: positionAt(about.content, start), End: positionAt(about.content, end)}
	for index, candidate := range candidates {
		item := completionItem{
			Label:      candidate.Label,
			Kind:       kindOf(candidate, about),
			Detail:     candidate.Detail,
			FilterText: candidate.Label,
			SortText:   fmt.Sprintf("%05d", index),
			TextEdit:   textEdit{Range: replaced, NewText: candidate.Label},
		}
		if candidate.Documentation != "" {
			item.Documentation = &markupContent{Kind: markdownKind, Value: candidate.Documentation}
		}
		items = append(items, item)
	}
	return items
}

func (s *Server) locationOf(root string, where project.Location) location {
	content, _ := os.ReadFile(filepath.FromSlash(root + "/" + where.File))
	s.state.Lock()
	if unsaved, isOpen := s.unsaved[where.File]; isOpen {
		content = unsaved
	}
	s.state.Unlock()
	return location{URI: uriOf(root + "/" + where.File), Range: rangeAt(content, where.Position.Line, where.Position.Column)}
}

func kindOf(candidate insight.Candidate, about asked) int {
	switch {
	case about.isOfSettings:
		return kindOfAProperty
	case about.isOfData && strings.HasPrefix(candidate.Label, "/"):
		return kindOfAFile
	case about.isOfData:
		return kindOfAValue
	}
	return kindOfARole[candidate.Role]
}

func theFileNamed(about asked) []location {
	named, isAFile := about.insight.FileNamedAt(about.content, about.offset)
	if !isAFile {
		return []location{}
	}
	if _, err := os.Stat(filepath.FromSlash(about.root + named)); err != nil {
		return []location{}
	}
	return []location{{URI: uriOf(about.root + named)}}
}

func (s *Server) definition(about asked) any {
	if named, isAFile := about.insight.FileNamedBySetting(about.content, about.offset); about.isOfSettings && isAFile {
		return []location{{URI: uriOf(about.root + named)}}
	}
	if about.isOfSettings {
		return []location{}
	}
	if about.isOfData {
		return theFileNamed(about)
	}
	found := about.insight.Define(about.file, about.content, about.offset)
	locations := make([]location, 0, len(found))
	for _, where := range found {
		if !about.insight.Project.IsOfALibrary(where.File) {
			locations = append(locations, s.locationOf(about.root, where))
		}
	}
	if len(locations) == 0 {
		return theFileNamed(about)
	}
	return locations
}

func (s *Server) hover(about asked) any {
	said, start, end, isSomewhere := about.insight.Describe(about.file, about.content, about.offset)
	switch {
	case about.isOfSettings:
		said, start, end, isSomewhere = about.insight.DescribeSetting(about.content, about.offset)
	case about.isOfData:
		said, start, end, isSomewhere = about.insight.DescribeFile(about.content, about.offset)
	}
	if !isSomewhere {
		return nil
	}
	return hover{
		Contents: markupContent{Kind: markdownKind, Value: said},
		Range:    textRange{Start: positionAt(about.content, start), End: positionAt(about.content, end)},
	}
}

func (s *Server) codeActions(about asked) any {
	actions := []codeAction{}
	for _, problem := range about.problems {
		if problem.Location.File != about.file || (problem.Suggestion == "" && !problem.RemovesTheLine) {
			continue
		}
		shown := rangeAt(about.content, problem.Location.Position.Line, problem.Location.Position.Column)
		if shown.End.Line < about.within.Start.Line || shown.Start.Line > about.within.End.Line {
			continue
		}
		if problem.RemovesTheLine {
			actions = append(actions, codeAction{
				Title:       titleOfRemovingALine,
				Kind:        quickFixKind,
				IsPreferred: true,
				Diagnostics: []diagnostic{diagnosticOf(problem, about.content)},
				Edit:        workspaceEdit{Changes: map[string][]textEdit{about.uri: {{Range: wholeLineAt(about.content, shown.Start.Line)}}}},
			})
			continue
		}
		start, end := boundsAt(about.content, problem.Location.Position.Line, problem.Location.Position.Column)
		if about.isOfData {
			start, end = valueInDataAt(about.content, start)
		}
		replaceStart, replaceEnd, replacement := whatToReplace(string(about.content[start:end]), problem.Suggestion)
		actions = append(actions, codeAction{
			Title:       fmt.Sprintf("Change to %q", replacement),
			Kind:        quickFixKind,
			IsPreferred: true,
			Diagnostics: []diagnostic{diagnosticOf(problem, about.content)},
			Edit: workspaceEdit{Changes: map[string][]textEdit{about.uri: {{
				Range:   textRange{Start: positionAt(about.content, start+replaceStart), End: positionAt(about.content, start+replaceEnd)},
				NewText: replacement,
			}}}},
		})
	}
	return actions
}

func valueInDataAt(content []byte, offset int) (start, end int) {
	for offset < len(content) && (content[offset] == '"' || content[offset] == '\\') {
		offset++
	}
	return insight.ValueAround(content, offset)
}

func wholeLineAt(content []byte, line int) textRange {
	start := offsetAt(content, position{Line: line})
	end := start + bytes.IndexByte(content[start:], '\n') + 1
	if end == start {
		end = len(content)
	}
	return textRange{Start: positionAt(content, start), End: positionAt(content, end)}
}

func whatToReplace(written, suggestion string) (start, end int, replacement string) {
	start, end = 0, len(written)
	if len(written) >= 2 && strings.ContainsRune(charactersOfAString, rune(written[0])) && written[len(written)-1] == written[0] {
		start, end = 1, len(written)-1
	}
	inner := written[start:end]
	if strings.HasPrefix(suggestion, startOfAnAbsolutePath) {
		pathStart := strings.Index(inner, startOfAnAbsolutePath)
		pathEnd := len(inner)
		if fragment := strings.IndexByte(inner, startOfAFragment); fragment >= 0 {
			pathEnd = fragment
		}
		if pathStart >= 0 && pathStart <= pathEnd {
			return start + pathStart, start + pathEnd, suggestion
		}
	}
	type part struct{ start, end int }
	var parts []part
	from := 0
	for index, character := range inner {
		if strings.ContainsRune(separatorsOfAnAddress, character) {
			parts = append(parts, part{from, index})
			from = index + utf8.RuneLen(character)
		}
	}
	parts = append(parts, part{from, len(inner)})
	if whole, _, hasAnElement := strings.Cut(inner[from:], separatorOfAnElement); hasAnElement {
		parts = append(parts, part{from, from + len(whole)})
	}
	named := strings.TrimLeft(suggestion, separatorsOfAnAddress)
	for _, part := range slices.Backward(parts) {
		candidate := inner[part.start:part.end]
		if _, isClose := suggest.Closest(candidate, []string{named}); isClose && candidate != "" && candidate != named {
			return start + part.start, start + part.end, named
		}
	}
	return start, end, suggestion
}
