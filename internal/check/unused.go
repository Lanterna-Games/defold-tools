package check

import (
	"path"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
	"github.com/Lanterna-Games/defold-tools/internal/source"
)

const (
	unusedFileCheck        = "unused-file"
	suffixOfACompiledFile  = "c"
	extensionOfALuaModule  = ".lua"
	functionThatLoadsLua   = "require"
	separatorOfAModuleName = "."
)

var extensionsOfFilesSomethingHasToUse = map[string]bool{
	".script": true, ".gui_script": true, ".render_script": true, ".lua": true,
	".go": true, ".gui": true, ".atlas": true, ".tilesource": true, ".tilemap": true,
	".sprite": true, ".collisionobject": true, ".particlefx": true, ".sound": true, ".font": true,
	".material": true, ".label": true, ".factory": true, ".collectionfactory": true, ".collectionproxy": true,
	".camera": true, ".model": true, ".mesh": true, ".spinescene": true, ".spinemodel": true, ".cubemap": true,
	".animationset": true, ".render": true, ".input_binding": true, ".display_profiles": true,
	".texture_profiles": true, ".gamepads": true,
}

type usage struct {
	usedBy                    map[string]string
	directoriesTheSettingsUse []string
	loadsLuaByANameItBuilds   bool
}

func (u *usage) use(resourcePath, by string) {
	if resourcePath != by && u.usedBy[resourcePath] == "" {
		u.usedBy[resourcePath] = by
	}
}

func (u *usage) usesWhatAStringNames(text, by string) {
	if strings.HasPrefix(text, "/") {
		u.use(text, by)
		u.use(strings.TrimSuffix(text, suffixOfACompiledFile), by)
	}
	u.use("/"+strings.ReplaceAll(text, separatorOfAModuleName, "/")+extensionOfALuaModule, by)
	u.use("/"+text, by)
}

func usageOf(loaded *project.Project) *usage {
	used := &usage{usedBy: map[string]string{}}
	for _, reference := range loaded.References {
		used.use(reference.Path, "/"+reference.Location.File)
	}
	for _, named := range loaded.PathsInSettings {
		used.use(named, "the settings")
		used.use(strings.TrimSuffix(named, suffixOfACompiledFile), "the settings")
		used.directoriesTheSettingsUse = append(used.directoriesTheSettingsUse, strings.TrimSuffix(named, "/")+"/")
	}
	used.use(loaded.GameBinding, "the settings")
	for _, scriptPath := range loaded.ScriptPaths() {
		script := loaded.Scripts[scriptPath]
		for _, token := range lua.Tokens(script.Source) {
			if token.Kind == lua.TokenString {
				used.usesWhatAStringNames(token.Text, scriptPath)
			}
		}
		lua.Inspect(script.Chunk, func(node lua.Node) bool {
			call, isCall := node.(*lua.CallExpression)
			if !isCall || len(call.Arguments) == 0 {
				return true
			}
			_, loadsByAWrittenName := call.Arguments[0].(*lua.StringLiteral)
			if name, _ := lua.QualifiedName(call.Function); name == functionThatLoadsLua && !loadsByAWrittenName {
				used.loadsLuaByANameItBuilds = true
			}
			return true
		})
	}
	return used
}

func (u *usage) isUsed(resourcePath string) bool {
	if u.usedBy[resourcePath] != "" {
		return true
	}
	for _, directory := range u.directoriesTheSettingsUse {
		if strings.HasPrefix(resourcePath, directory) {
			return true
		}
	}
	return false
}

func unusedFiles(inspected inspection) []Problem {
	loaded := inspected.project
	if loaded.IsALibrary {
		return nil
	}
	used := usageOf(loaded)
	var problems []Problem
	for _, resourcePath := range loaded.Files() {
		extension := path.Ext(resourcePath)
		isLuaThatMayBeLoadedByName := extension == extensionOfALuaModule && used.loadsLuaByANameItBuilds
		if !extensionsOfFilesSomethingHasToUse[extension] || isLuaThatMayBeLoadedByName || used.isUsed(resourcePath) {
			continue
		}
		problems = append(problems, Problem{
			Check:    unusedFileCheck,
			Location: project.Location{File: strings.TrimPrefix(resourcePath, "/"), Position: source.Position{Line: 1, Column: 1}},
			Message:  "nothing in the project uses this file",
		})
	}
	return problems
}
