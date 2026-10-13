package insight

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const (
	SettingsFile           = "game.project"
	manualOfTheSettings    = "https://defold.com/manuals/project-settings/"
	separatorOfAListedItem = "#"
)

type lineOfSettings struct {
	section      string
	start        int
	text         string
	isASection   bool
	key          string
	keyStart     int
	keyEnd       int
	hasAValue    bool
	valueStart   int
	valueEnd     int
	cursorInLine int
}

func lineOfSettingsAt(content []byte, offset int) lineOfSettings {
	offset = min(max(offset, 0), len(content))
	start := bytes.LastIndexByte(content[:offset], '\n') + 1
	end := len(content)
	if next := bytes.IndexByte(content[offset:], '\n'); next >= 0 {
		end = offset + next
	}
	line := lineOfSettings{start: start, text: strings.TrimRight(string(content[start:end]), "\r"), cursorInLine: offset - start}
	for before := range strings.SplitSeq(string(content[:start]), "\n") {
		if trimmed := strings.TrimSpace(before); strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			line.section = strings.Trim(trimmed, "[]")
		}
	}
	indent := len(line.text) - len(strings.TrimLeft(line.text, " \t"))
	if strings.HasPrefix(strings.TrimSpace(line.text), "[") {
		line.isASection = true
		line.keyStart = start + indent + 1
		line.keyEnd = start + len(strings.TrimRight(strings.TrimRight(line.text, " \t"), "]"))
		line.key = string(content[line.keyStart:max(line.keyEnd, line.keyStart)])
		return line
	}
	written, value, hasAValue := strings.Cut(line.text, "=")
	line.keyStart = start + indent
	line.keyEnd = start + len(strings.TrimRight(written, " \t"))
	line.key, _, _ = strings.Cut(strings.TrimSpace(written), separatorOfAListedItem)
	line.hasAValue = hasAValue
	if hasAValue {
		line.valueStart = start + len(written) + 1 + len(value) - len(strings.TrimLeft(value, " \t"))
		line.valueEnd = start + len(strings.TrimRight(line.text, " \t"))
		line.valueEnd = max(line.valueEnd, line.valueStart)
	}
	return line
}

func describedSetting(section, key string, setting engine.Setting) string {
	lines := []string{fmt.Sprintf("**%s.%s**", section, key)}
	if setting.Type != "" {
		lines[0] += " · " + setting.Type
	}
	if setting.Help != "" {
		lines = append(lines, "", strings.ToUpper(setting.Help[:1])+setting.Help[1:]+".")
	}
	if setting.Default != "" {
		lines = append(lines, "", "Default: `"+setting.Default+"`")
	}
	if len(setting.Options) > 0 {
		lines = append(lines, "", "One of: `"+strings.Join(setting.Options, "`, `")+"`")
	}
	if setting.IsDeprecated {
		lines = append(lines, "", "Deprecated.")
	}
	return strings.Join(append(lines, "", "[Project settings]("+manualOfTheSettings+")"), "\n")
}

func (i Insight) CompleteSetting(content []byte, offset int) (candidates []Candidate, start, end int) {
	line := lineOfSettingsAt(content, offset)
	schema := i.Reference.Settings
	switch {
	case line.isASection:
		for _, name := range sortedKeys(schema) {
			candidates = append(candidates, Candidate{Label: name, Detail: "section", Documentation: schema[name].Help})
		}
		return candidates, line.keyStart, max(line.keyEnd, line.keyStart)
	case !line.hasAValue || offset <= line.keyEnd:
		var written []string
		for _, set := range i.Project.Settings {
			if set.Section == line.section && !set.IsASection {
				written = append(written, set.Key)
			}
		}
		for _, key := range sortedKeys(schema[line.section].Settings) {
			setting := schema[line.section].Settings[key]
			if !slices.Contains(written, key) || key == line.key {
				candidates = append(candidates, Candidate{Label: key, Detail: setting.Type, Documentation: describedSetting(line.section, key, setting)})
			}
		}
		return candidates, line.keyStart, max(line.keyEnd, line.keyStart)
	}
	setting := schema[line.section].Settings[line.key]
	values := setting.Options
	if setting.Type == engine.SettingIsABool {
		values = []string{"0", "1"}
	}
	for _, value := range values {
		candidates = append(candidates, Candidate{Label: value, Detail: line.section + "." + line.key})
	}
	return candidates, line.valueStart, line.valueEnd
}

func (i Insight) DescribeSetting(content []byte, offset int) (markdown string, start, end int, isSomewhere bool) {
	line := lineOfSettingsAt(content, offset)
	if offset < line.keyStart || offset > line.keyEnd || line.key == "" {
		return "", 0, 0, false
	}
	if line.isASection {
		section, isKnown := i.Reference.Settings[line.key]
		if !isKnown || section.Help == "" {
			return "", 0, 0, false
		}
		return fmt.Sprintf("**[%s]**\n\n%s.\n\n[Project settings](%s)", line.key, section.Help, manualOfTheSettings), line.keyStart, line.keyEnd, true
	}
	if removedIn, wasRemoved := i.Reference.WhenTheSettingWasRemoved(line.section, line.key); wasRemoved {
		said := fmt.Sprintf("**%s.%s**\n\nDefold %s removed this setting: the line is no longer read.\n\n[Project settings](%s)",
			line.section, line.key, removedIn, manualOfTheSettings)
		return said, line.keyStart, line.keyEnd, true
	}
	setting, isKnown := i.Reference.Settings[line.section].Settings[line.key]
	if !isKnown {
		return "", 0, 0, false
	}
	return describedSetting(line.section, line.key, setting), line.keyStart, line.keyEnd, true
}

func (i Insight) FileNamedBySetting(content []byte, offset int) (resourcePath string, isAFile bool) {
	line := lineOfSettingsAt(content, offset)
	if !line.hasAValue || offset < line.valueStart || offset > line.valueEnd {
		return "", false
	}
	value := string(content[line.valueStart:line.valueEnd])
	for _, candidate := range []string{value, strings.TrimSuffix(value, "c")} {
		if i.Project.HasFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}
