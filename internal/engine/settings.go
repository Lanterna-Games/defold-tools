package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	SettingIsABool     = "bool"
	SettingIsAnInteger = "integer"
	SettingIsANumber   = "number"
	SettingIsAResource = "resource"

	settingsSchemaPath        = "%s/%s/com.dynamo.cr/com.dynamo.cr.bob/src/com/dynamo/bob/meta.properties"
	separatorOfAnAttribute    = "."
	separatorOfOptions        = ","
	separatorOfAnOptionsLabel = ":"
	wordForYes                = "1"
)

type Setting struct {
	Type              string   `json:"type,omitempty"`
	Default           string   `json:"default,omitempty"`
	Help              string   `json:"help,omitempty"`
	Options           []string `json:"options,omitempty"`
	Minimum           *float64 `json:"minimum,omitempty"`
	Maximum           *float64 `json:"maximum,omitempty"`
	IsDeprecated      bool     `json:"deprecated,omitempty"`
	KeepsItsExtension bool     `json:"keeps_extension,omitempty"`
}

type SettingsSection struct {
	Help     string             `json:"help,omitempty"`
	Settings map[string]Setting `json:"settings,omitempty"`
}

func boundIn(written string) *float64 {
	if bound, err := strconv.ParseFloat(written, 64); err == nil {
		return &bound
	}
	return nil
}

func (s *Setting) take(attribute, value string) {
	switch attribute {
	case "type":
		s.Type = value
	case "default":
		s.Default = value
	case "help":
		s.Help = value
	case "options":
		for option := range strings.SplitSeq(value, separatorOfOptions) {
			name, _, _ := strings.Cut(option, separatorOfAnOptionsLabel)
			s.Options = append(s.Options, strings.TrimSpace(name))
		}
	case "minimum":
		s.Minimum = boundIn(value)
	case "maximum":
		s.Maximum = boundIn(value)
	case "deprecated":
		s.IsDeprecated = value == wordForYes
	case "preserve-extension":
		s.KeepsItsExtension = value == wordForYes
	}
}

func SettingsFrom(schema []byte) map[string]SettingsSection {
	sections := map[string]SettingsSection{}
	name := ""
	lines := bufio.NewScanner(bytes.NewReader(schema))
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "[") {
			name = strings.Trim(line, "[]")
			if name != "" {
				sections[name] = SettingsSection{Settings: map[string]Setting{}}
			}
			continue
		}
		written, value, isSet := strings.Cut(line, "=")
		if !isSet || name == "" || strings.HasPrefix(line, "#") {
			continue
		}
		written, value = strings.TrimSpace(written), strings.TrimSpace(value)
		key, attribute, isOfAKey := strings.Cut(written, separatorOfAnAttribute)
		section := sections[name]
		if !isOfAKey {
			if written == "help" {
				section.Help = value
				sections[name] = section
			}
			continue
		}
		setting := section.Settings[key]
		setting.take(attribute, value)
		section.Settings[key] = setting
	}
	return sections
}

func RememberSettingsOfOtherVersions(references []*Reference) {
	ordered := slices.Clone(references)
	slices.SortFunc(ordered, func(a, b *Reference) int {
		left, _ := ParseVersion(a.Defold)
		right, _ := ParseVersion(b.Defold)
		return left.Compare(right)
	})
	hasIt := map[string]map[string][]bool{}
	for index, reference := range ordered {
		for name, section := range reference.Settings {
			if hasIt[name] == nil {
				hasIt[name] = map[string][]bool{}
			}
			for key := range section.Settings {
				if hasIt[name][key] == nil {
					hasIt[name][key] = make([]bool, len(ordered))
				}
				hasIt[name][key][index] = true
			}
		}
	}
	for index, reference := range ordered {
		reference.SettingsElsewhere = map[string][]string{}
		reference.SettingsRemoved = map[string]map[string]string{}
		for name, keys := range hasIt {
			for key, where := range keys {
				if where[index] {
					continue
				}
				reference.SettingsElsewhere[name] = append(reference.SettingsElsewhere[name], key)
				lastBefore := lastIndexOf(where[:index], true)
				if lastBefore < 0 || slices.Contains(where[index:], true) {
					continue
				}
				if reference.SettingsRemoved[name] == nil {
					reference.SettingsRemoved[name] = map[string]string{}
				}
				reference.SettingsRemoved[name][key] = ordered[lastBefore+1].Defold
			}
			slices.Sort(reference.SettingsElsewhere[name])
		}
	}
}

func lastIndexOf(values []bool, wanted bool) int {
	for index, value := range slices.Backward(values) {
		if value == wanted {
			return index
		}
	}
	return -1
}

func (r *Reference) IsASettingOfAnotherVersion(section, key string) bool {
	return slices.Contains(r.SettingsElsewhere[section], key)
}

func (r *Reference) WhenTheSettingWasRemoved(section, key string) (version string, wasRemoved bool) {
	version, wasRemoved = r.SettingsRemoved[section][key]
	return version, wasRemoved
}

func (p Publisher) SettingsSchema(fetched func(url string) ([]byte, error), version string) ([]byte, error) {
	return fetched(fmt.Sprintf(settingsSchemaPath, p.Sources, version))
}
