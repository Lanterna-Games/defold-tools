package insight

import (
	"cmp"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
	"github.com/Lanterna-Games/defold-tools/internal/project"
	"github.com/Lanterna-Games/defold-tools/internal/source"
)

const (
	suffixOfACompiledName = "c"
	extensionOfAModule    = ".lua"
	separatorOfAModule    = "."
	separatorOfAPath      = "/"
)

var (
	ErrNothingToMove = errors.New("nothing of the project is there to move")

	extensionsOfScripts     = []string{".script", ".gui_script", ".render_script", ".lua", ".editor_script"}
	extensionsThatNameFiles = []string{
		".project", ".settings", ".go", ".collection", ".gui", ".atlas", ".tilemap", ".tilesource", ".tileset", ".sprite", ".label",
		".collisionobject", ".convexshape", ".collectionfactory", ".collectionproxy", ".factory", ".particlefx", ".model", ".mesh",
		".sound", ".font", ".material", ".render", ".render_target", ".camera", ".spinemodel", ".spinescene", ".cubemap",
		".display_profiles", ".input_binding", ".gamepads", ".texture_profiles", ".animationset", ".light", ".compute", ".appmanifest",
	}
	moduleThatIsLoaded = regexp.MustCompile(`\brequire\s*\(?\s*["']([^"'\\\r\n]+)["']`)
)

type Move struct {
	From string
	To   string
}

type writtenPath struct {
	offset   int
	text     string
	isModule bool
}

func ModuleNameOf(resourcePath string) string {
	withoutExtension := strings.TrimSuffix(strings.TrimPrefix(resourcePath, separatorOfAPath), extensionOfAModule)
	return strings.ReplaceAll(withoutExtension, separatorOfAPath, separatorOfAModule)
}

func comesBeforeAPath(character byte) bool {
	return strings.IndexByte("\"'\\ \t=,(\r\n", character) >= 0
}

func pathsWrittenIn(content []byte, isAScript bool) []writtenPath {
	var found []writtenPath
	for start := 0; start < len(content); start++ {
		if content[start] != '/' || (start > 0 && !comesBeforeAPath(content[start-1])) {
			continue
		}
		enders := " \t,)\"'\\\r\n"
		if start > 0 && (content[start-1] == '"' || content[start-1] == '\'') {
			enders = "\"'\\\r\n"
		}
		end := start
		for end < len(content) && strings.IndexByte(enders, content[end]) < 0 {
			end++
		}
		if end-start > 1 {
			found = append(found, writtenPath{offset: start, text: string(content[start:end])})
		}
		start = end
	}
	if isAScript {
		for _, loaded := range moduleThatIsLoaded.FindAllSubmatchIndex(content, -1) {
			found = append(found, writtenPath{offset: loaded[2], text: string(content[loaded[2]:loaded[3]]), isModule: true})
		}
	}
	return found
}

func (i Insight) isAFileOfTheProject(resourcePath string) bool {
	return i.Project.HasFile(resourcePath)
}

func (i Insight) holdsFiles(directory string) bool {
	return slices.ContainsFunc(i.Project.Files(), func(file string) bool { return strings.HasPrefix(file, directory+separatorOfAPath) })
}

func (m Move) of(written writtenPath, isAFile bool) (moved string, isMoved bool) {
	from, to, text := m.From, m.To, written.text
	if written.isModule {
		from, to = ModuleNameOf(m.From), ModuleNameOf(m.To)
		switch {
		case isAFile && path.Ext(m.From) != extensionOfAModule:
			return "", false
		case text == from && isAFile:
			return to, true
		case !isAFile && strings.HasPrefix(text, from+separatorOfAModule):
			return to + strings.TrimPrefix(text, from), true
		}
		return "", false
	}
	switch {
	case text == from:
		return to, true
	case isAFile && text == from+suffixOfACompiledName:
		return to + suffixOfACompiledName, true
	case !isAFile && strings.HasPrefix(text, from+separatorOfAPath):
		return to + strings.TrimPrefix(text, from), true
	}
	return "", false
}

func isReadForPaths(file string) (isRead, isAScript bool) {
	extension := path.Ext(file)
	isAScript = slices.Contains(extensionsOfScripts, extension)
	return isAScript || slices.Contains(extensionsThatNameFiles, extension), isAScript
}

func (i Insight) ChangesToMove(moves []Move, contentOf func(file string) ([]byte, error)) ([]Change, error) {
	isAFile := map[Move]bool{}
	var known []Move
	for _, move := range moves {
		move.From, move.To = strings.TrimSuffix(move.From, separatorOfAPath), strings.TrimSuffix(move.To, separatorOfAPath)
		switch {
		case move.From == move.To || move.From == "":
			continue
		case i.isAFileOfTheProject(move.To) || i.holdsFiles(move.To):
			return nil, fmt.Errorf("%s is already there", move.To)
		case i.isAFileOfTheProject(move.From):
			isAFile[move] = true
		case !i.holdsFiles(move.From):
			continue
		}
		known = append(known, move)
	}
	if len(known) == 0 {
		return nil, ErrNothingToMove
	}
	var changes []Change
	for _, file := range i.Project.Files() {
		isRead, isAScript := isReadForPaths(file)
		if !isRead || i.Project.IsOfALibrary(strings.TrimPrefix(file, separatorOfAPath)) {
			continue
		}
		relative := strings.TrimPrefix(file, separatorOfAPath)
		content, err := contentOf(relative)
		if err != nil {
			continue
		}
		var locate source.Locator
		for _, written := range pathsWrittenIn(content, isAScript) {
			for _, move := range known {
				moved, isMoved := move.of(written, isAFile[move])
				if !isMoved {
					continue
				}
				if locate == nil {
					locate = source.NewLocator(content)
				}
				changes = append(changes, Change{Where: project.Location{File: relative, Position: locate(written.offset)}, Old: written.text, New: moved})
				break
			}
		}
	}
	slices.SortFunc(changes, func(first, second Change) int {
		return cmp.Or(cmp.Compare(first.Where.File, second.Where.File), cmp.Compare(first.Where.Position.Offset, second.Where.Position.Offset))
	})
	return changes, nil
}

func AnimationsRenamedBy(changes []Change) []string {
	var renamed []string
	for _, change := range changes {
		was, is := atlas.NameOfAnImage(change.Old, ""), atlas.NameOfAnImage(change.New, "")
		if path.Ext(change.Where.File) == atlas.Extension && slices.Contains(atlas.ExtensionsOfImages, strings.ToLower(path.Ext(change.Old))) && was != is {
			renamed = append(renamed, fmt.Sprintf("%q of /%s becomes %q", was, change.Where.File, is))
		}
	}
	return renamed
}

func (i Insight) PlacesThatName(resourcePath string, contentOf func(file string) ([]byte, error)) []Change {
	var places []Change
	isAFile := i.isAFileOfTheProject(resourcePath)
	named := Move{From: resourcePath, To: resourcePath}
	for _, file := range i.Project.Files() {
		isRead, isAScript := isReadForPaths(file)
		if !isRead {
			continue
		}
		relative := strings.TrimPrefix(file, separatorOfAPath)
		content, err := contentOf(relative)
		if err != nil {
			continue
		}
		var locate source.Locator
		for _, written := range pathsWrittenIn(content, isAScript) {
			if _, isNamed := named.of(written, isAFile); !isNamed {
				continue
			}
			if locate == nil {
				locate = source.NewLocator(content)
			}
			places = append(places, Change{Where: project.Location{File: relative, Position: locate(written.offset)}, Old: written.text, New: written.text})
		}
	}
	return places
}
