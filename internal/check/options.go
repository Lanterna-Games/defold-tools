package check

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	ignoreCheck            = "ignore"
	ignoreDirective        = "defold-vet:ignore"
	separatorOfChecks      = ","
	anyNumberOfDirectories = "**"
	separatorOfDirectories = "/"
)

var names = []string{
	missingResourceCheck, syntaxCheck, unassignedSelfFieldCheck, unboundActionCheck, undeclaredPropertyCheck,
	unhandledMessageCheck, unknownAnimationCheck, unknownChildCheck, unknownCollisionGroupCheck,
	unknownGUINodeCheck, unknownMessageFieldCheck, unknownPropertyCheck, unpostedMessageCheck, unresolvedURLCheck,
	unusedFileCheck, unverifiableCheck, unknownSettingCheck, invalidSettingCheck, unsetSettingCheck,
	removedSettingCheck,
}

var checksOfUpkeep = map[string]bool{unverifiableCheck: true, unusedFileCheck: true, removedSettingCheck: true}

func IsAMistake(name string) bool {
	return !checksOfUpkeep[name]
}

func Names() []string {
	return slices.Clone(names)
}

func IsACheck(name string) bool {
	return slices.Contains(names, name)
}

type Ignore struct {
	Check  string
	Path   string
	Reason string
}

type Options struct {
	TurnedOff map[string]bool
	Ignores   []Ignore
}

func (o Options) silences(problem Problem) bool {
	if o.TurnedOff[problem.Check] {
		return true
	}
	for _, ignore := range o.Ignores {
		if ignore.Check == problem.Check && MatchesPath(ignore.Path, problem.Location.File) {
			return true
		}
	}
	return false
}

func MatchesPath(pattern, file string) bool {
	return matchesSegments(strings.Split(pattern, separatorOfDirectories), strings.Split(file, separatorOfDirectories))
}

func matchesSegments(pattern, file []string) bool {
	if len(pattern) == 0 {
		return len(file) == 0
	}
	if pattern[0] == anyNumberOfDirectories {
		for skipped := 0; skipped <= len(file); skipped++ {
			if matchesSegments(pattern[1:], file[skipped:]) {
				return true
			}
		}
		return false
	}
	if len(file) == 0 {
		return false
	}
	if matched, err := path.Match(pattern[0], file[0]); err != nil || !matched {
		return false
	}
	return matchesSegments(pattern[1:], file[1:])
}

type lineIgnore struct {
	checks      []string
	location    project.Location
	lineItIsFor int
	isUsed      map[string]bool
}

func (l *lineIgnore) covers(problem Problem) bool {
	isOnItsLine := problem.Location.File == l.location.File && problem.Location.Position.Line == l.lineItIsFor
	return isOnItsLine && slices.Contains(l.checks, problem.Check)
}

func lineIgnoresIn(loaded *project.Project) (ignores []*lineIgnore, malformed []Problem) {
	for _, scriptPath := range loaded.ScriptPaths() {
		script := loaded.Scripts[scriptPath]
		for _, comment := range script.Chunk.Comments {
			written, isADirective := strings.CutPrefix(strings.TrimSpace(comment.Text), ignoreDirective)
			if !isADirective {
				continue
			}
			location := project.Location{File: script.File, Position: comment.Start}
			checks, reason, _ := strings.Cut(strings.TrimSpace(written), " ")
			ignore := &lineIgnore{location: location, lineItIsFor: comment.Start.Line, isUsed: map[string]bool{}}
			if comment.IsAloneOnItsLine {
				ignore.lineItIsFor = comment.End.Line + 1
			}
			for check := range strings.SplitSeq(checks, separatorOfChecks) {
				switch {
				case check == "":
				case !IsACheck(check):
					malformed = append(malformed, Problem{Check: ignoreCheck, Location: location,
						Message: fmt.Sprintf("%q is not a check%s", check, hintTowards(check, names, "%q"))})
				default:
					ignore.checks = append(ignore.checks, check)
				}
			}
			switch {
			case checks == "":
				malformed = append(malformed, Problem{Check: ignoreCheck, Location: location, Message: "this ignore names no check"})
			case strings.TrimSpace(reason) == "":
				malformed = append(malformed, Problem{Check: ignoreCheck, Location: location,
					Message: fmt.Sprintf("this ignore gives no reason: write why after %q", checks)})
			default:
				ignores = append(ignores, ignore)
			}
		}
	}
	return ignores, malformed
}

func (o Options) apply(loaded *project.Project, found []Problem) []Problem {
	lineIgnores, kept := lineIgnoresIn(loaded)
	for _, problem := range found {
		isIgnoredOnItsLine := false
		for _, ignore := range lineIgnores {
			if ignore.covers(problem) {
				ignore.isUsed[problem.Check] = true
				isIgnoredOnItsLine = true
			}
		}
		if !isIgnoredOnItsLine && !o.silences(problem) {
			kept = append(kept, problem)
		}
	}
	for _, ignore := range lineIgnores {
		for _, check := range ignore.checks {
			if !ignore.isUsed[check] && !o.TurnedOff[check] {
				kept = append(kept, Problem{Check: ignoreCheck, Location: ignore.location,
					Message: fmt.Sprintf("nothing is reported here for %q: this ignore can go", check)})
			}
		}
	}
	return kept
}
