package conventions

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	startOfAClass  = regexp.MustCompile(`^class (\w+)`)
	methodOfAClass = regexp.MustCompile(`^  (?:async )?(\w+)\([^)]*\) \{$`)
)

// The extension is plain JavaScript with no build step and nothing that lints
// it. A class there may define a method twice without a word from anyone, and
// the second quietly replaces the first. It happened: stopping the game took
// the name of stopping the language server, so a restart left the old server
// running and every completion and fix was offered twice. It was found by
// looking at a running VS Code, which is too late and too rare a way to find it.
func TestNoClassOfTheExtensionDefinesAMethodTwice(t *testing.T) {
	directory := filepath.Join(repositoryRoot(t), "editors", "vscode")
	sources, err := filepath.Glob(filepath.Join(directory, "*.js"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no JavaScript was found in %s: %v", directory, err)
	}
	classes := 0
	for _, source := range sources {
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		class := ""
		defined := map[string]int{}
		for number, line := range strings.Split(string(content), "\n") {
			if found := startOfAClass.FindStringSubmatch(line); found != nil {
				class, defined = found[1], map[string]int{}
				classes++
				continue
			}
			if line == "}" {
				class = ""
			}
			found := methodOfAClass.FindStringSubmatch(line)
			if class == "" || found == nil {
				continue
			}
			if first, isDefined := defined[found[1]]; isDefined {
				t.Errorf("%s: %s.%s is defined on line %d and again on line %d, and the second replaces the first", filepath.Base(source), class, found[1], first, number+1)
			}
			defined[found[1]] = number + 1
		}
	}
	if classes == 0 {
		t.Error("no class was found in the extension, so this test looked at nothing")
	}
}
