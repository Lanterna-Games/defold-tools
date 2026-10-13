// Package conventions holds the rules of this codebase that a linter does not
// enforce, written as tests so that they fail the build instead of a review.
package conventions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Source files explain themselves through names and structure. The reasoning
// behind a decision lives in docs/decisions, in a test name or in the commit
// message, where it does not drift from the code it sits next to.
//
// Three kinds of comment survive, because something reads them:
//   - directives for the toolchain and the linter (//go:build, //nolint:...);
//   - the package comment in a doc.go, which is what `go doc` shows;
//   - anything in a test file, where a comment says why a case exists.
func TestSourceFilesCarryNoComments(t *testing.T) {
	root := repositoryRoot(t)
	files := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if isSkippedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCheckedSourceFile(entry.Name()) {
			return nil
		}

		parsed, err := parser.ParseFile(files, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		if ast.IsGenerated(parsed) {
			return nil
		}
		for _, group := range parsed.Comments {
			if isPackageCommentOfDocFile(parsed, group, entry.Name()) {
				continue
			}
			for _, comment := range group.List {
				if isDirective(comment.Text) {
					continue
				}
				position := files.Position(comment.Pos())
				relative, _ := filepath.Rel(root, position.Filename)
				t.Errorf("%s:%d: comment in a source file: %s", filepath.ToSlash(relative), position.Line, firstLine(comment.Text))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isSkippedDirectory(name string) bool {
	return name == "testdata" || name == "vendor" || name == "build" || strings.HasPrefix(name, ".")
}

func isCheckedSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func isPackageCommentOfDocFile(file *ast.File, group *ast.CommentGroup, fileName string) bool {
	return fileName == "doc.go" && group == file.Doc
}

func isDirective(comment string) bool {
	for _, prefix := range []string{"//go:", "//nolint:", "//line "} {
		if strings.HasPrefix(comment, prefix) {
			return true
		}
	}
	return false
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("go.mod not found above the test's working directory")
		}
		directory = parent
	}
}
