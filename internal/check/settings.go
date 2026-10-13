package check

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
	"github.com/Lanterna-Games/defold-tools/internal/source"
)

const (
	unknownSettingCheck      = "unknown-setting"
	invalidSettingCheck      = "invalid-setting"
	unsetSettingCheck        = "unset-setting"
	removedSettingCheck      = "removed-setting"
	directoryOfTheEngine     = "/builtins/"
	settingsFileOfTheProject = "game.project"
)

var settingsABuildCannotDoWithout = []string{"bootstrap.main_collection", "input.game_binding"}

func sortedNames[V any](named map[string]V) []string {
	names := make([]string, 0, len(named))
	for name := range named {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func withoutItsNumbers(key string) string {
	return strings.Map(func(character rune) rune {
		if character >= '0' && character <= '9' {
			return -1
		}
		return character
	}, key)
}

func whyTheValueIsNotOne(setting engine.Setting, value string) (reason string, isWrong bool) {
	number, err := strconv.ParseFloat(value, 64)
	switch setting.Type {
	case engine.SettingIsABool:
		if value != "0" && value != "1" {
			return "is not 0 or 1", true
		}
	case engine.SettingIsAnInteger:
		if _, notAnInteger := strconv.ParseInt(value, 10, 64); notAnInteger != nil {
			return "is not a whole number", true
		}
	case engine.SettingIsANumber:
		if err != nil {
			return "is not a number", true
		}
	default:
		return "", false
	}
	if setting.Minimum != nil && err == nil && number < *setting.Minimum {
		return fmt.Sprintf("is below the minimum of %v", *setting.Minimum), true
	}
	if setting.Maximum != nil && err == nil && number > *setting.Maximum {
		return fmt.Sprintf("is above the maximum of %v", *setting.Maximum), true
	}
	return "", false
}

func unknownSettings(inspected inspection) []Problem {
	loaded, schema := inspected.project, inspected.reference.Settings
	if len(schema) == 0 {
		return nil
	}
	var problems []Problem
	for _, written := range loaded.Settings {
		section, isKnown := schema[written.Section]
		removedIn, wasRemoved := inspected.reference.WhenTheSettingWasRemoved(written.Section, written.Key)
		switch {
		case !written.IsASection && wasRemoved:
			problems = append(problems, Problem{Check: removedSettingCheck, Location: written.KeyWhere, RemovesTheLine: true,
				Message: fmt.Sprintf("%q is no longer a setting of [%s]: Defold %s removed it", written.Key, written.Section, removedIn)})
		case written.IsASection && !isKnown:
			if hint := hintTowards(written.Section, sortedNames(schema), "%q"); hint != "" {
				problems = append(problems, Problem{Check: unknownSettingCheck, Location: written.SectionWhere,
					Message: fmt.Sprintf("%q is not a section of the settings%s", written.Section, hint)})
			}
		case written.IsASection || !isKnown:
		default:
			setting, isASetting := section.Settings[written.Key]
			if !isASetting && inspected.reference.IsASettingOfAnotherVersion(written.Section, written.Key) {
				continue
			}
			if !isASetting {
				known := sortedNames(section.Settings)
				isAnotherSizeOfOne := slices.ContainsFunc(known, func(key string) bool { return withoutItsNumbers(key) == withoutItsNumbers(written.Key) })
				if hint := hintTowards(written.Key, known, "%q"); hint != "" && !isAnotherSizeOfOne {
					problems = append(problems, Problem{Check: unknownSettingCheck, Location: written.KeyWhere,
						Message: fmt.Sprintf("%q is not a setting of [%s]%s", written.Key, written.Section, hint)})
				}
				continue
			}
			problems = append(problems, valueProblems(loaded, written, setting)...)
		}
	}
	return problems
}

func unsetSettings(inspected inspection) []Problem {
	loaded, schema := inspected.project, inspected.reference.Settings
	if len(loaded.Settings) == 0 {
		return nil
	}
	isWritten := map[string]bool{}
	for _, written := range loaded.Settings {
		if !written.IsASection && written.Value != "" {
			isWritten[written.Section+"."+written.Key] = true
		}
	}
	var problems []Problem
	for _, section := range sortedNames(schema) {
		for _, key := range sortedNames(schema[section].Settings) {
			setting := schema[section].Settings[key]
			fallback := strings.TrimSuffix(setting.Default, suffixOfACompiledFile)
			isOfTheEngine := strings.HasPrefix(fallback, directoryOfTheEngine)
			isNeededToBuild := slices.Contains(settingsABuildCannotDoWithout, section+"."+key)
			if !isNeededToBuild || setting.Type != engine.SettingIsAResource || isOfTheEngine || isWritten[section+"."+key] || loaded.HasFile(fallback) {
				continue
			}
			problems = append(problems, Problem{
				Check:    unsetSettingCheck,
				Location: project.Location{File: settingsFileOfTheProject, Position: source.Position{Line: 1, Column: 1}},
				Message:  fmt.Sprintf("%s.%s is not set, and %q, which it falls back on, does not exist", section, key, fallback),
			})
		}
	}
	return problems
}

func valueProblems(loaded *project.Project, written project.SettingWritten, setting engine.Setting) []Problem {
	if written.Value == "" {
		return nil
	}
	if setting.Type == engine.SettingIsAResource {
		source := strings.TrimSuffix(written.Value, suffixOfACompiledFile)
		isThere := loaded.HasFile(written.Value) || loaded.HasFile(source)
		if isThere || !strings.HasPrefix(written.Value, "/") || !loaded.HasTopLevelDirectoryOf(written.Value) {
			return nil
		}
		return []Problem{{Check: missingResourceCheck, Location: written.ValueWhere, Message: fmt.Sprintf("%q does not exist", written.Value)}}
	}
	if reason, isWrong := whyTheValueIsNotOne(setting, written.Value); isWrong {
		return []Problem{{Check: invalidSettingCheck, Location: written.ValueWhere,
			Message: fmt.Sprintf("%q %s, which is what %s.%s takes", written.Value, reason, written.Section, written.Key)}}
	}
	return nil
}
