package libraries

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha1" //nolint:gosec // the name Defold gives a fetched library is the SHA-1 of its address, not a secret
	"encoding/hex"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const (
	Directory           = ".internal/lib"
	archiveExtension    = ".zip"
	settingsOfALibrary  = "game.project"
	sectionOfALibrary   = "[library]"
	keyOfTheDirectories = "include_dirs"
	separatorOfTheTag   = "-"

	sectionOfTheProject    = "[project]"
	keyOfTheDependencies   = "dependencies"
	separatorOfAListedItem = "#"
)

type Mounted struct {
	fs.FS
	Directories []string
	fetched     map[string]bool
}

func keyOf(address string) string {
	digest := sha1.Sum([]byte(address)) //nolint:gosec // as above
	return hex.EncodeToString(digest[:])
}

func (m Mounted) Owns(filePath string) bool {
	top, _, _ := strings.Cut(strings.TrimPrefix(filePath, "/"), "/")
	return slices.Contains(m.Directories, top)
}

func (m Mounted) HasAll(addresses []string) bool {
	for _, address := range addresses {
		if !m.fetched[keyOf(address)] {
			return false
		}
	}
	return true
}

func listedUnder(settings []byte, wantedSection, wantedKey string) []string {
	var listed []string
	section := ""
	lines := bufio.NewScanner(bytes.NewReader(settings))
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		key, value, isSet := strings.Cut(line, "=")
		name, _, _ := strings.Cut(strings.TrimSpace(key), separatorOfAListedItem)
		if isSet && section == wantedSection && name == wantedKey {
			listed = append(listed, strings.FieldsFunc(value, func(character rune) bool { return character == ',' || character == ' ' || character == '\t' })...)
		}
	}
	return listed
}

func directoriesOf(archive *zip.Reader) (base string, directories []string, isALibrary bool) {
	for _, file := range archive.File {
		if path.Base(file.Name) != settingsOfALibrary {
			continue
		}
		content, err := fs.ReadFile(archive, file.Name)
		if err != nil {
			return "", nil, false
		}
		return path.Dir(file.Name), listedUnder(content, sectionOfALibrary, keyOfTheDirectories), true
	}
	return "", nil, false
}

func Mount(files fs.FS) Mounted {
	mounted := Mounted{FS: files, fetched: map[string]bool{}}
	entries, err := fs.ReadDir(files, Directory)
	settings, errOfSettings := fs.ReadFile(files, settingsOfALibrary)
	if err != nil || errOfSettings != nil {
		return mounted
	}
	isDependedOn := map[string]bool{}
	for _, address := range listedUnder(settings, sectionOfTheProject, keyOfTheDependencies) {
		isDependedOn[keyOf(address)] = true
	}
	mounts := map[string]fs.FS{}
	for _, entry := range entries {
		key, _, _ := strings.Cut(entry.Name(), separatorOfTheTag)
		if entry.IsDir() || path.Ext(entry.Name()) != archiveExtension || !isDependedOn[key] {
			continue
		}
		content, err := fs.ReadFile(files, path.Join(Directory, entry.Name()))
		if err != nil {
			continue
		}
		archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			continue
		}
		base, directories, isALibrary := directoriesOf(archive)
		if !isALibrary {
			continue
		}
		mounted.fetched[key] = true
		for _, directory := range directories {
			if _, isOfTheProject := fs.Stat(files, directory); isOfTheProject == nil || mounts[directory] != nil {
				continue
			}
			if inside, err := fs.Sub(archive, path.Join(base, directory)); err == nil {
				mounts[directory] = inside
				mounted.Directories = append(mounted.Directories, directory)
			}
		}
	}
	slices.Sort(mounted.Directories)
	if len(mounts) > 0 {
		mounted.FS = withLibraries{base: files, mounts: mounts}
	}
	return mounted
}

type withLibraries struct {
	base   fs.FS
	mounts map[string]fs.FS
}

func (w withLibraries) whereIs(name string) (files fs.FS, inside string) {
	top, rest, isBelow := strings.Cut(name, "/")
	mount, isMounted := w.mounts[top]
	switch {
	case !isMounted:
		return w.base, name
	case !isBelow:
		return mount, "."
	default:
		return mount, rest
	}
}

func (w withLibraries) Open(name string) (fs.File, error) {
	files, inside := w.whereIs(name)
	return files.Open(inside)
}

func (w withLibraries) ReadDir(name string) ([]fs.DirEntry, error) {
	files, inside := w.whereIs(name)
	entries, err := fs.ReadDir(files, inside)
	if err != nil || name != "." {
		return entries, err
	}
	for directory, mount := range w.mounts {
		if info, err := fs.Stat(mount, "."); err == nil {
			entries = append(entries, mountedDirectory{name: directory, FileInfo: info})
		}
	}
	slices.SortFunc(entries, func(left, right fs.DirEntry) int { return strings.Compare(left.Name(), right.Name()) })
	return entries, nil
}

type mountedDirectory struct {
	fs.FileInfo
	name string
}

func (m mountedDirectory) Name() string               { return m.name }
func (m mountedDirectory) Type() fs.FileMode          { return fs.ModeDir }
func (m mountedDirectory) Info() (fs.FileInfo, error) { return m, nil }
