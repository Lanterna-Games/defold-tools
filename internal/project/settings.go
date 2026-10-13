package project

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/source"
)

const (
	defaultGameBinding     = "/input/game.input_binding"
	suffixOfACompiledFile  = "c"
	separatorOfAListedItem = "#"
	separatorOfListedPaths = ","
	defaultMainCollection  = "/logic/main.collection"
	sectionOfTheProject    = "project"
	keyOfTheDependencies   = "dependencies"
)

var extensionsOfSettings = map[string]bool{".project": true, ".settings": true}

type settings map[string]map[string]string

type SettingWritten struct {
	Section      string
	Key          string
	Value        string
	KeyWhere     Location
	ValueWhere   Location
	SectionWhere Location
	IsASection   bool
}

func settingsWrittenIn(filePath string, content []byte) []SettingWritten {
	var written []SettingWritten
	locate := source.NewLocator(content)
	section, sectionWhere := "", Location{}
	offset := 0
	for line := range bytes.SplitSeq(content, []byte("\n")) {
		start := offset
		offset += len(line) + 1
		text := string(line)
		indent := len(text) - len(strings.TrimLeft(text, " \t"))
		trimmed := strings.TrimSpace(text)
		switch {
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			section = strings.Trim(trimmed, "[]")
			sectionWhere = Location{File: filePath, Position: locate(start + indent + 1)}
			written = append(written, SettingWritten{Section: section, SectionWhere: sectionWhere, IsASection: true})
		case trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#"):
		default:
			key, value, isSetting := strings.Cut(text, "=")
			if !isSetting {
				continue
			}
			name, _, _ := strings.Cut(strings.TrimSpace(key), separatorOfAListedItem)
			valueStart := start + len(key) + 1 + len(value) - len(strings.TrimLeft(value, " \t"))
			written = append(written, SettingWritten{
				Section: section, Key: name, Value: strings.TrimSpace(value), SectionWhere: sectionWhere,
				KeyWhere:   Location{File: filePath, Position: locate(start + indent)},
				ValueWhere: Location{File: filePath, Position: locate(valueStart)},
			})
		}
	}
	return written
}

func settingsFrom(content []byte) settings {
	parsed := settings{}
	section := ""
	lines := bufio.NewScanner(bytes.NewReader(content))
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, isSetting := strings.Cut(line, "=")
		if !isSetting || strings.TrimSpace(value) == "" {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(key), separatorOfAListedItem)
		if parsed[section] == nil {
			parsed[section] = map[string]string{}
		}
		parsed[section][name] = strings.TrimSpace(value)
	}
	return parsed
}

func separatesAddresses(character rune) bool {
	return character == ',' || character == ' ' || character == '\t'
}

func (s settings) isOfALibrary() bool {
	return s["library"]["include_dirs"] != ""
}

func (s settings) pathsNamed() []string {
	var paths []string
	for _, section := range s {
		for _, value := range section {
			for item := range strings.SplitSeq(value, separatorOfListedPaths) {
				if named := strings.TrimSpace(item); named != "" && !strings.ContainsAny(named, charactersNoPathHolds) {
					paths = append(paths, "/"+strings.TrimPrefix(named, "/"))
				}
			}
		}
	}
	return paths
}

func (s settings) gameBinding() string {
	compiled := s["input"]["game_binding"]
	if compiled == "" {
		return defaultGameBinding
	}
	return strings.TrimSuffix(compiled, suffixOfACompiledFile)
}
