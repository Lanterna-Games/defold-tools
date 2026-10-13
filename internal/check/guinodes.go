package check

import (
	"fmt"
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const unknownGUINodeCheck = "unknown-gui-node"

type sceneNodes struct {
	scene *project.GUIScene
	nodes project.GUINodes
}

type guiNodeJudge struct {
	loaded                 *project.Project
	everyScene             []sceneNodes
	namedSomewhere         []string
	aModuleNamesUnreadably bool
	anyoneNamesUnreadably  bool
}

func unknownGUINodes(inspected inspection) []Problem {
	loaded := inspected.project
	judge := guiNodeJudge{loaded: loaded}
	for _, scenePath := range sortedScenePaths(loaded) {
		scene := loaded.GUIScenes[scenePath]
		judge.everyScene = append(judge.everyScene, sceneNodes{scene: scene, nodes: loaded.NodesOf(scene)})
	}
	for _, script := range loaded.Scripts {
		judge.namedSomewhere = append(judge.namedSomewhere, script.GUINodesItNames...)
		if script.NamesGUINodesThatCannotBeRead {
			judge.anyoneNamesUnreadably = true
			judge.aModuleNamesUnreadably = judge.aModuleNamesUnreadably || script.IsALuaModule()
		}
	}

	var problems []Problem
	for _, scriptPath := range loaded.ScriptPaths() {
		script := loaded.Scripts[scriptPath]
		for _, lookup := range script.GUINodesLookedUp {
			if rejection, isRejected := judge.whyTheNodeIsNotThere(script, lookup.Name); isRejected {
				problems = append(problems, Problem{Check: unknownGUINodeCheck, Location: lookup.Location, Message: rejection})
			}
		}
	}
	return problems
}

func sortedScenePaths(loaded *project.Project) []string {
	paths := make([]string, 0, len(loaded.GUIScenes))
	for scenePath := range loaded.GUIScenes {
		paths = append(paths, scenePath)
	}
	slices.Sort(paths)
	return paths
}

func (j guiNodeJudge) scenesRunning(script *project.Script) []sceneNodes {
	var scenes []sceneNodes
	for _, candidate := range j.everyScene {
		if candidate.scene.Script == script.Path {
			scenes = append(scenes, candidate)
		}
	}
	return scenes
}

func (j guiNodeJudge) whyTheNodeIsNotThere(script *project.Script, id string) (rejection string, isRejected bool) {
	scenes := j.scenesRunning(script)
	runsInScenesThatAreKnown := len(scenes) > 0
	if !runsInScenesThatAreKnown {
		if j.loaded.RunsCodeOutOfSight || j.anyoneNamesUnreadably || slices.Contains(j.namedSomewhere, id) {
			return "", false
		}
		scenes = j.everyScene
	} else if script.NamesGUINodesThatCannotBeRead || j.aModuleNamesUnreadably || slices.Contains(script.GUINodesItNames, id) {
		return "", false
	}
	if len(scenes) == 0 {
		return "", false
	}

	var known []string
	for _, candidate := range scenes {
		if candidate.nodes.Has(id) || candidate.nodes.CouldHoldMoreThanWasRead(id) {
			return "", false
		}
		known = append(known, candidate.nodes.IDs()...)
	}
	switch {
	case !runsInScenesThatAreKnown:
		rejection = fmt.Sprintf("%q: no GUI scene in the project has a node %q", id, id)
	case len(scenes) == 1:
		rejection = fmt.Sprintf("%q: %s has no node %q", id, scenes[0].scene.Path, id)
	default:
		rejection = fmt.Sprintf("%q: none of the %d scenes that run this script has a node %q", id, len(scenes), id)
	}
	return rejection + hintTowards(id, known, "%q"), true
}
