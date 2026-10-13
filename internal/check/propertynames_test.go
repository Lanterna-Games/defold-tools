package check

import "testing"

func TestPropertiesThatExist(t *testing.T) {
	files := pongWithScripts(`go.property("speed", 300)
go.property("direction", vmath.vector3())

local TINT = hash("tint")

function update(self, dt)
    go.set(".", "position.x", 0)
    go.animate(".", "euler.z", go.PLAYBACK_LOOP_FORWARD, 360, go.EASING_LINEAR, 1)
    go.set("/game", "scale", 2)
    go.set("#sprite", "tint.w", 0.5)
    go.set("#sprite", TINT, vmath.vector4())
    go.set("#sprite", "glow", vmath.vector4())
    self.texture = go.get("#sprite", "texture0")
    go.cancel_animations("#sprite", "cursor")
    self.speed = go.get("#", "speed") + go.get("#ball", "direction.x")
    self.score = go.get("/game#game", "score")
    self.other = go.get(self.target, "speed")
    self.zoom = go.get(self.camera, "orthographic_zoom")
    go.set(self.target, "glow.x", 1)
    go.set(self.target, self.property_name, 1)
end
`, `go.property("score", 0)
`)
	files["main/glow.material"] = `name: "glow"
fragment_constants {
  name: "glow"
  type: CONSTANT_TYPE_USER
}
`

	assertNoProblems(t, problemsOf(problemsIn(t, files), unknownPropertyCheck))
	assertNoProblems(t, problemsOf(problemsIn(t, files), undeclaredPropertyCheck))
}

func TestPropertyAGameObjectDoesNotHave(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function update(self, dt)
    go.set(".", "pozition.x", 0)
    go.animate("/game", "tint", go.PLAYBACK_ONCE_FORWARD, vmath.vector4(), go.EASING_LINEAR, 1)
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownPropertyCheck, "main/ball.script", 2,
		`"pozition.x": "." is a game object, which has no property "pozition"`, `did you mean "position"?`)
	assertOneProblem(t, problems[1:], unknownPropertyCheck, "main/ball.script", 3,
		`"tint": "/game" is a game object, which has no property "tint"`)
}

func TestPropertyAComponentDoesNotHave(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function update(self, dt)
    go.set("#sprite", "tnit.w", 0)
end
`))

	assertOneProblem(t, problems, unknownPropertyCheck, "main/ball.script", 2,
		`"tnit.w": "#sprite" is a sprite component, which has no property "tnit"`, `did you mean "tint"?`)
}

func TestPropertyAScriptDoesNotDeclare(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    self.score = go.get("/game#game", "scroe")
    self.lives = go.get("/game#game", "lives")
end
`, `go.property("score", 0)
go.property("level", 1)
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], undeclaredPropertyCheck, "main/ball.script", 2,
		`"scroe": /main/game.script does not declare the property "scroe"`, `did you mean "score"?`)
	assertOneProblem(t, problems[1:], undeclaredPropertyCheck, "main/ball.script", 3,
		`/main/game.script does not declare the property "lives"; it declares "score" and "level"`)
}

func TestPropertyOfAnAddressThatIsNotWrittenDown(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function update(self, dt)
    go.set(self.target, "pozition", vmath.vector3())
    go.set(self.target, "position", vmath.vector3())
end
`+"\n"))

	assertOneProblem(t, problemsOf(problems, unknownPropertyCheck), unknownPropertyCheck, "main/ball.script", 2,
		`"pozition": nothing in the project has a property "pozition"`, `did you mean "position"?`)
}

func TestALuaModuleIsJudgedAgainstEveryProperty(t *testing.T) {
	files := pongWithScripts("", "go.property(\"score\", 0)\n")
	files["main/tween.lua"] = `local M = {}

function M.fade(url)
    go.animate(url, "tint.w", go.PLAYBACK_ONCE_FORWARD, 0, go.EASING_LINEAR, 1)
    go.animate(url, "score", go.PLAYBACK_ONCE_FORWARD, 0, go.EASING_LINEAR, 1)
    go.animate(url, "alpha", go.PLAYBACK_ONCE_FORWARD, 0, go.EASING_LINEAR, 1)
end

return M
`

	assertOneProblem(t, problemsIn(t, files), unknownPropertyCheck, "main/tween.lua", 6, `nothing in the project has a property "alpha"`)
}

// A component the reference says nothing about may have any property, and so may
// a script whose declarations cannot all be read.
func TestHoldersThatMayHaveMoreThanIsKnownAreLeftAlone(t *testing.T) {
	files := pongWithScripts(`function update(self, dt)
    go.set("#spine", "skin", hash("red"))
    go.set("/game#game", "anything", 1)
end
`, `local name = "sco" .. "re"
go.property(name, 0)
`)
	files["main/ball.go"] = ballWithSpriteAndScript + `embedded_components {
  id: "spine"
  type: "spinemodel"
  data: ""
}
`

	assertNoProblems(t, problemsIn(t, files))
}

func TestPropertiesAreNotJudgedAgainstEverythingWhenCodeIsOutOfSight(t *testing.T) {
	files := pongWithBallScript(`function update(self, dt)
    go.set(self.target, "outline_width", 2)
    go.set("#sprite", "outline_width", 2)
    go.set(".", "outline_width", 2)
end
`)
	files["game.project"] = settingsWithALibrary

	assertOneProblem(t, problemsOf(problemsIn(t, files), unknownPropertyCheck), unknownPropertyCheck, "main/ball.script", 4,
		`"." is a game object`)
}

func TestAPropertyOfAGUIScriptItselfIsOneOfTheGUIComponent(t *testing.T) {
	files := pongWithScripts("", "")
	files["main/game.go"] = gameWithScript + `components {
  id: "hud"
  component: "/main/hud.gui"
}
`
	files["main/hud.gui"] = "script: \"/main/hud.gui_script\"\n"
	files["main/hud.gui_script"] = `function init(self)
    self.fonts = go.get("#", "fonts", { key = "main" })
    self.speed = go.get("#", "speed")
end
`

	assertOneProblem(t, problemsIn(t, files), unknownPropertyCheck, "main/hud.gui_script", 3,
		`"#" is a gui component, which has no property "speed"`)
}

// The same script on two game objects is right if it is right on one of them.
func TestAPropertyThatExistsOnOneOfTheGameObjectsIsNotReported(t *testing.T) {
	files := pongWithScripts(`function update(self, dt)
    go.set("#sprite", "gain", 1)
    go.set("#sprite", "volume", 1)
end
`, "")
	files["main/game.go"] = `components {
  id: "game"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sound"
  data: ""
}
`

	assertOneProblem(t, problemsOf(problemsIn(t, files), unknownPropertyCheck), unknownPropertyCheck, "main/ball.script", 3, `"volume"`)
}
