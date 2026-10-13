package config

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/Lanterna-Games/defold-tools/internal/check"
)

const FileName = "defold-vet.toml"

const strictModeCheck = "unverifiable"

type Config struct {
	Defold  string          `toml:"defold"`
	Strict  *bool           `toml:"strict"`
	Checks  map[string]bool `toml:"checks"`
	Ignores []Ignore        `toml:"ignore"`
	Known   Known           `toml:"known"`
}

type Known struct {
	Messages   []string `toml:"messages"`
	Properties []string `toml:"properties"`
}

type Ignore struct {
	Check  string `toml:"check"`
	Path   string `toml:"path"`
	Reason string `toml:"reason"`
}

func Load(files fs.FS) (*Config, error) {
	content, err := fs.ReadFile(files, FileName)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", FileName, err)
	}
	var loaded Config
	metadata, err := toml.Decode(string(content), &loaded)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if mistakes := append(unknownSettingsIn(metadata), loaded.mistakes()...); len(mistakes) > 0 {
		return nil, fmt.Errorf("%s: %s", FileName, strings.Join(mistakes, "; "))
	}
	return &loaded, nil
}

func unknownSettingsIn(metadata toml.MetaData) []string {
	var mistakes []string
	for _, key := range metadata.Undecoded() {
		mistakes = append(mistakes, fmt.Sprintf("%q is not a setting", key.String()))
	}
	return mistakes
}

func (c *Config) mistakes() []string {
	var mistakes []string
	for _, name := range sortedNames(c.Checks) {
		if !check.IsACheck(name) {
			mistakes = append(mistakes, fmt.Sprintf("%q is not a check", name))
		}
	}
	for index, ignore := range c.Ignores {
		entry := fmt.Sprintf("ignore %d", index+1)
		switch {
		case ignore.Check == "" || ignore.Path == "" || strings.TrimSpace(ignore.Reason) == "":
			mistakes = append(mistakes, entry+" needs a check, a path and a reason")
		case !check.IsACheck(ignore.Check):
			mistakes = append(mistakes, fmt.Sprintf("%s: %q is not a check", entry, ignore.Check))
		}
	}
	return mistakes
}

func sortedNames(checks map[string]bool) []string {
	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) Options() check.Options {
	options := check.Options{TurnedOff: map[string]bool{}}
	if c.Strict != nil {
		options.TurnedOff[strictModeCheck] = !*c.Strict
	}
	for name, isOn := range c.Checks {
		options.TurnedOff[name] = !isOn
	}
	for _, ignore := range c.Ignores {
		options.Ignores = append(options.Ignores, check.Ignore{Check: ignore.Check, Path: ignore.Path, Reason: ignore.Reason})
	}
	return options
}
