package libraries

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

const address = "https://github.com/britzl/monarch/archive/refs/tags/5.2.0.zip"

func archiveOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var written bytes.Buffer
	archive := zip.NewWriter(&written)
	for name, content := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return written.Bytes()
}

func walked(t *testing.T, files fs.FS) []string {
	t.Helper()
	var paths []string
	err := fs.WalkDir(files, ".", func(filePath string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filePath[0] != '.' {
			paths = append(paths, filePath)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	return paths
}

// Defold keeps a fetched library in .internal/lib, in an archive named after the SHA-1
// of the address it was fetched from, and mounts the directories its game.project lists
// at the root of the project. The name below is what Defold gives that address.
func TestAFetchedLibraryIsMountedWhereDefoldMountsIt(t *testing.T) {
	library := archiveOf(t, map[string]string{
		"monarch-5.2.0/game.project":        "[project]\ntitle = Monarch\n\n[library]\ninclude_dirs = monarch, shared\n",
		"monarch-5.2.0/monarch/monarch.lua": "return {}\n",
		"monarch-5.2.0/shared/util.lua":     "return {}\n",
		"monarch-5.2.0/example/main.script": "",
	})
	tree := fstest.MapFS{
		"game.project":     {Data: []byte("[project]\ntitle = Game\ndependencies#0 = " + address + "\n")},
		"main/main.script": {Data: []byte("")},
		"shared/own.lua":   {Data: []byte("return {}\n")},
		".internal/lib/" + keyOf(address) + "-IjEyMyI=.zip":                   {Data: library},
		".internal/lib/" + keyOf("https://example.com/old.zip") + "-IjEi.zip": {Data: library},
		".internal/lib/notes.txt":                                             {Data: []byte("not an archive")},
	}

	mounted := Mount(tree)

	if keyOf(address) != "eb3bd90096e68da57610da6b67a25c9345fc53a1" {
		t.Errorf("the key of an address is %q, want its SHA-1 in hexadecimal, as sha1sum gives it", keyOf(address))
	}
	if !slices.Equal(mounted.Directories, []string{"monarch"}) {
		t.Errorf("directories = %q, want the one of the library the project does not have itself", mounted.Directories)
	}
	if got, want := walked(t, mounted.FS), []string{"game.project", "main/main.script", "monarch/monarch.lua", "shared/own.lua"}; !slices.Equal(got, want) {
		t.Errorf("files = %q, want %q", got, want)
	}
	if content, err := fs.ReadFile(mounted.FS, "monarch/monarch.lua"); err != nil || string(content) != "return {}\n" {
		t.Errorf("monarch.lua reads %q, %v", content, err)
	}
	if !mounted.Owns("monarch/monarch.lua") || !mounted.Owns("/monarch/monarch.lua") || mounted.Owns("main/main.script") || mounted.Owns("shared/own.lua") {
		t.Error("what belongs to the library is not told apart from what belongs to the project")
	}
	if !mounted.HasAll([]string{address}) || mounted.HasAll([]string{address, "https://example.com/missing.zip"}) || mounted.HasAll([]string{"https://example.com/old.zip"}) {
		t.Error("the libraries that were fetched are not told apart from those that were not")
	}
}

func TestAProjectWithoutFetchedLibrariesIsLeftAsItIs(t *testing.T) {
	tree := fstest.MapFS{"game.project": {Data: []byte("[project]\ndependencies = " + address + "\n")}, "main/main.script": {Data: []byte("")}}

	mounted := Mount(tree)

	if len(mounted.Directories) != 0 || mounted.HasAll([]string{address}) || !mounted.HasAll(nil) {
		t.Errorf("mounted = %+v, want nothing mounted and nothing fetched", mounted.Directories)
	}
	if got := walked(t, mounted.FS); !slices.Equal(got, []string{"game.project", "main/main.script"}) {
		t.Errorf("files = %q", got)
	}
}
