package check

import (
	"fmt"
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const unknownMessageFieldCheck = "unknown-message-field"

type postedFields struct {
	names           []string
	couldHoldOthers bool
}

type fieldJudge struct {
	inspection
	postedIn             map[string]*postedFields
	everyPostIsInSight   bool
	everyPostHasAKnownID bool
}

func unknownMessageFields(inspected inspection) []Problem {
	loaded := inspected.project
	judge := fieldJudge{
		inspection:           inspected,
		postedIn:             map[string]*postedFields{},
		everyPostIsInSight:   !loaded.RunsCodeOutOfSight,
		everyPostHasAKnownID: true,
	}
	for _, script := range loaded.Scripts {
		if !script.IsReadInFull {
			judge.everyPostIsInSight = false
		}
		for _, post := range script.Posts {
			if !post.IDIsKnown {
				judge.everyPostHasAKnownID = false
				continue
			}
			posted, isSeen := judge.postedIn[post.ID.Name]
			if !isSeen {
				posted = &postedFields{}
				judge.postedIn[post.ID.Name] = posted
			}
			posted.names = append(posted.names, post.Fields...)
			posted.couldHoldOthers = posted.couldHoldOthers || !post.FieldsAreWritten
		}
	}

	var problems []Problem
	for _, scriptPath := range loaded.ScriptPaths() {
		for _, read := range loaded.Scripts[scriptPath].FieldsRead {
			if rejection, isRejected := judge.whyTheFieldIsNotThere(read); isRejected {
				problems = append(problems, Problem{Check: unknownMessageFieldCheck, Location: read.Field.Location, Message: rejection})
			}
		}
	}
	return problems
}

func (j fieldJudge) fieldsOf(message string) (fields []string, areAllKnown, isSentByTheEngine bool) {
	if documented, isSentByTheEngine := j.reference.FieldsOfWhatTheEngineSends(message); isSentByTheEngine {
		return documented, true, true
	}
	posted, isPosted := j.postedIn[message]
	if !j.everyPostIsInSight || !j.everyPostHasAKnownID || !isPosted || posted.couldHoldOthers {
		return nil, false, false
	}
	return posted.names, true, false
}

func (j fieldJudge) whyTheFieldIsNotThere(read project.FieldRead) (rejection string, isRejected bool) {
	var known []string
	everyMessageIsSentByTheEngine := true
	for _, message := range read.Messages {
		fields, areAllKnown, isSentByTheEngine := j.fieldsOf(message)
		if !areAllKnown || slices.Contains(fields, read.Field.Name) {
			return "", false
		}
		known = append(known, fields...)
		everyMessageIsSentByTheEngine = everyMessageIsSentByTheEngine && isSentByTheEngine
	}
	if len(read.Messages) == 0 {
		return "", false
	}
	hint := hintTowards(read.Field.Name, known, "%q")
	if everyMessageIsSentByTheEngine {
		return fmt.Sprintf("%q is not a field of %s%s", read.Field.Name, messagesNamed(read.Messages), hint), true
	}
	return fmt.Sprintf("%q is read from %s, but nothing that posts it sets %q%s",
		read.Field.Name, messagesNamed(read.Messages), read.Field.Name, hint), true
}

func messagesNamed(messages []string) string {
	unique := slices.Compact(slices.Sorted(slices.Values(messages)))
	if len(unique) == 1 {
		return fmt.Sprintf("%q", unique[0])
	}
	return "any of " + quotedList(unique)
}
