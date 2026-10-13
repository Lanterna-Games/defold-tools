package check

import "testing"

const hudWithAScoreAndAButton = `script: "/main/hud.gui_script"
nodes {
  id: "score"
  type: TYPE_TEXT
}
nodes {
  id: "restart"
  type: TYPE_TEMPLATE
  template: "/main/button.gui"
}
nodes {
  id: "restart/label"
  type: TYPE_TEXT
  parent: "restart/background"
  template_node_child: true
}
`

const buttonTemplate = `nodes {
  id: "background"
  type: TYPE_BOX
}
nodes {
  id: "label"
  type: TYPE_TEXT
}
nodes {
  id: "icon"
  type: TYPE_TEMPLATE
  template: "/main/icon.gui"
}
`

const iconTemplate = `nodes {
  id: "image"
  type: TYPE_BOX
}
`

func pongWithHUD(guiScript string) map[string]string {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/hud.gui"] = hudWithAScoreAndAButton
	files["main/button.gui"] = buttonTemplate
	files["main/icon.gui"] = iconTemplate
	files["main/hud.gui_script"] = guiScript
	return files
}

func TestGUINodesThatExist(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithHUD(`local SCORE = hash("score")

function init(self)
    self.score = gui.get_node("score")
    self.same = gui.get_node(SCORE)
    self.label = gui.get_node("restart/label")
    self.background = gui.get_node(hash("restart/background"))
    self.image = gui.get_node("restart/icon/image")
    self.dynamic = gui.get_node("item_" .. self.index)
end
`+"function update(self) self.index = 1 end\n")))
}

func TestGUINodeTheSceneDoesNotHave(t *testing.T) {
	problems := problemsIn(t, pongWithHUD(`function init(self)
    self.score = gui.get_node("scroe")
    self.label = gui.get_node("restart/lable")
    self.other = gui.get_node("label")
end
`))

	if len(problems) != 3 {
		t.Fatalf("got %d problems, want 3: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownGUINodeCheck, "main/hud.gui_script", 2,
		`"scroe": /main/hud.gui has no node "scroe"`, `did you mean "score"?`)
	assertOneProblem(t, problems[1:2], unknownGUINodeCheck, "main/hud.gui_script", 3, `did you mean "restart/label"?`)
	assertOneProblem(t, problems[2:], unknownGUINodeCheck, "main/hud.gui_script", 4, `/main/hud.gui has no node "label"`)
}

// A template that comes from a library cannot be read. Whatever is asked for
// under the node that holds it may be there.
func TestNodesUnderATemplateThatCouldNotBeReadAreLeftAlone(t *testing.T) {
	files := pongWithHUD(`function init(self)
    self.label = gui.get_node("restart/whatever")
    self.other = gui.get_node("restrat/whatever")
end
`)
	delete(files, "main/button.gui")

	assertOneProblem(t, problemsOf(problemsIn(t, files), unknownGUINodeCheck), unknownGUINodeCheck, "main/hud.gui_script", 3, `"restrat/whatever"`)
}

func TestANodeTheScriptNamesItselfIsNotExpectedInTheScene(t *testing.T) {
	problems := problemsIn(t, pongWithHUD(`function init(self)
    gui.set_id(gui.new_box_node(vmath.vector3(), vmath.vector3()), "marker")
    self.marker = gui.get_node("marker")
    self.other = gui.get_node("marke")
end
`))

	assertOneProblem(t, problems, unknownGUINodeCheck, "main/hud.gui_script", 4, `"marke"`)
}

func TestAScriptThatNamesNodesInWaysThatCannotBeReadIsNotJudged(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithHUD(`function init(self)
    for index = 1, 3 do
        gui.set_id(gui.new_box_node(vmath.vector3(), vmath.vector3()), "marker_" .. index)
    end
    self.marker = gui.get_node("marker_1")
end
`)))
}

// The same script in two scenes is right if it is right in one of them.
func TestAGUIScriptIsJudgedAgainstEverySceneThatRunsIt(t *testing.T) {
	files := pongWithHUD(`function init(self)
    self.score = gui.get_node("score")
    self.title = gui.get_node("title")
    self.nothing = gui.get_node("nothing")
end
`)
	files["main/menu.gui"] = `script: "/main/hud.gui_script"
nodes {
  id: "title"
  type: TYPE_TEXT
}
`

	assertOneProblem(t, problemsIn(t, files), unknownGUINodeCheck, "main/hud.gui_script", 4,
		`"nothing": none of the 2 scenes that run this script has a node "nothing"`)
}

func TestALuaModuleIsJudgedAgainstEveryScene(t *testing.T) {
	files := pongWithHUD("")
	files["main/widgets.lua"] = `local M = {}

function M.score()
    return gui.get_node("score"), gui.get_node("restart/icon/image"), gui.get_node("hiscore")
end

return M
`

	assertOneProblem(t, problemsIn(t, files), unknownGUINodeCheck, "main/widgets.lua", 4,
		`"hiscore": no GUI scene in the project has a node "hiscore"`)
}

func TestALuaModuleIsNotJudgedWhenScenesMayComeFromOutside(t *testing.T) {
	files := pongWithHUD("")
	files["main/widgets.lua"] = `return { score = function() return gui.get_node("hiscore") end }
`
	files["game.project"] = settingsWithALibrary

	assertNoProblems(t, problemsIn(t, files))
}
