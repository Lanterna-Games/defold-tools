package lsp

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/insight"
)

const (
	methodThatMovesFiles = "workspace/willRenameFiles"
	methodThatSaysUses   = "defold/uses"
	everyFile            = "**/*"
	schemeOfAFile        = "file"
)

func (s *Server) insightNow() (known insight.Insight, root string, isReady bool) {
	s.state.Lock()
	defer s.state.Unlock()
	if s.analysed.Project == nil {
		return insight.Insight{}, "", false
	}
	return insight.Insight{Project: s.analysed.Project, Reference: s.analysed.Reference, Documentation: s.documentation}, s.root, true
}

func (s *Server) placesOf(root string, places []insight.Change) []location {
	locations := make([]location, 0, len(places))
	for _, place := range places {
		content, err := s.contentOf(root, place.Where.File)
		if err != nil || !place.Fits(content) {
			continue
		}
		start := place.Where.Position.Offset
		locations = append(locations, location{
			URI:   uriOf(root + "/" + place.Where.File),
			Range: textRange{Start: positionAt(content, start), End: positionAt(content, start+len(place.Old))},
		})
	}
	return locations
}

func (s *Server) moveFiles(received *message) error {
	var params renameFilesParams
	if err := json.Unmarshal(received.Params, &params); err != nil {
		return s.connection.refuse(received.ID, requestWasInvalid, err.Error())
	}
	known, root, isReady := s.insightNow()
	var moves []insight.Move
	for _, renamed := range params.Files {
		from, isFromTheProject := s.relative(renamed.OldURI)
		to, isToTheProject := s.relative(renamed.NewURI)
		if isFromTheProject && isToTheProject {
			moves = append(moves, insight.Move{From: "/" + from, To: "/" + to})
		}
	}
	if !isReady || len(moves) == 0 {
		return s.connection.answer(received.ID, nil)
	}
	changes, err := known.ChangesToMove(moves, func(file string) ([]byte, error) { return s.contentOf(root, file) })
	switch {
	case errors.Is(err, insight.ErrNothingToMove):
		return s.connection.answer(received.ID, nil)
	case err != nil:
		s.warn("The files that name what is moved were left as they are: " + err.Error())
		return s.connection.answer(received.ID, nil)
	}
	edits := map[string][]textEdit{}
	for _, change := range changes {
		content, err := s.contentOf(root, change.Where.File)
		if err != nil || !change.Fits(content) {
			s.warn("The files that name what is moved were left as they are: " + change.Where.File + " changed underneath.")
			return s.connection.answer(received.ID, nil)
		}
		start := change.Where.Position.Offset
		uri := uriOf(root + "/" + change.Where.File)
		edits[uri] = append(edits[uri], textEdit{
			Range:   textRange{Start: positionAt(content, start), End: positionAt(content, start+len(change.Old))},
			NewText: change.New,
		})
	}
	if renamed := insight.AnimationsRenamedBy(changes); len(renamed) > 0 {
		s.warn("An image gives its name to its animation, so this move renames animations: " + strings.Join(renamed, ", ") + ". What plays them was left as it is.")
	}
	return s.connection.answer(received.ID, workspaceEdit{Changes: edits})
}

func (s *Server) usesOf(received *message) error {
	var params positionParams
	if err := json.Unmarshal(received.Params, &params); err != nil {
		return s.connection.refuse(received.ID, requestWasInvalid, err.Error())
	}
	known, root, isReady := s.insightNow()
	file, isInTheProject := s.relative(params.TextDocument.URI)
	if !isReady || !isInTheProject {
		return s.connection.answer(received.ID, []location{})
	}
	places := known.PlacesThatName("/"+file, func(named string) ([]byte, error) { return s.contentOf(root, named) })
	return s.connection.answer(received.ID, s.placesOf(root, places))
}
