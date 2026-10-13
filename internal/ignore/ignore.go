package ignore

import (
	"bufio"
	"bytes"
	"io/fs"
	"regexp"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const (
	FileName             = ".defignore"
	separator            = "/"
	charactersOfAPattern = "*?"
	anyDirectories       = "**"
	oneCharacter         = '?'
	anyCharacters        = '*'
)

var FirstDefoldWithPatterns = engine.Version{Major: 1, Minor: 14, Patch: 0}

type Rules struct {
	paths    []string
	patterns []*regexp.Regexp
}

func RulesOf(files fs.FS, defold engine.Version) Rules {
	content, err := fs.ReadFile(files, FileName)
	if err != nil {
		return Rules{}
	}
	var rules Rules
	lines := bufio.NewScanner(bytes.NewReader(content))
	for lines.Scan() {
		line := strings.TrimSuffix(lines.Text(), "\r")
		if !strings.HasPrefix(line, separator) {
			continue
		}
		written := strings.Trim(strings.ReplaceAll(line, `\`, separator), separator)
		if written == "" {
			continue
		}
		written = separator + written
		if strings.ContainsAny(written, charactersOfAPattern) && !defold.IsBefore(FirstDefoldWithPatterns) {
			rules.patterns = append(rules.patterns, expressionOf(written))
			continue
		}
		rules.paths = append(rules.paths, written)
	}
	return rules
}

func expressionOf(pattern string) *regexp.Regexp {
	pattern = strings.TrimSuffix(pattern, separator+anyDirectories)
	var expression, literal strings.Builder
	expression.WriteString("^")
	for index := 0; index < len(pattern); index++ {
		character := pattern[index]
		if character != anyCharacters && character != oneCharacter {
			literal.WriteByte(character)
			continue
		}
		expression.WriteString(regexp.QuoteMeta(literal.String()))
		literal.Reset()
		rest := pattern[index:]
		switch {
		case character == oneCharacter:
			expression.WriteString("[^/]")
		case strings.HasPrefix(rest, anyDirectories+separator):
			expression.WriteString("(?:.*/)?")
			index += len(anyDirectories)
		case strings.HasPrefix(rest, anyDirectories):
			expression.WriteString(".*")
			index++
		default:
			expression.WriteString("[^/]*")
		}
	}
	expression.WriteString(regexp.QuoteMeta(literal.String()))
	expression.WriteString("(?:/.*)?$")
	return regexp.MustCompile(expression.String())
}

func (r Rules) Hides(resourcePath string) bool {
	for _, ignored := range r.paths {
		if resourcePath == ignored || strings.HasPrefix(resourcePath, ignored+separator) {
			return true
		}
	}
	for _, pattern := range r.patterns {
		if pattern.MatchString(resourcePath) {
			return true
		}
	}
	return false
}
