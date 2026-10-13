package check

import "testing"

func TestAddressKeptInAConstantIsReportedWhereItIsWritten(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`local GAME = "/game#gaem"

function init(self)
    msg.post(GAME, "disable")
    msg.post(GAME, "enable")
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 1,
		`"/game#gaem": "/game" has no component "gaem"`, `did you mean "game"?`)
}

func TestURLBuiltFromAnAddressThatDoesNotExist(t *testing.T) {
	built := map[string]struct {
		script string
		want   string
	}{
		"one string": {
			script: `function init(self) self.sprite = msg.url("#sprit") end`,
			want:   `"#sprit": "/ball" has no component "sprit"`,
		},
		"three parts": {
			script: `function init(self) self.game = msg.url(nil, "/gaem", "game") end`,
			want:   `"/gaem#game": collection "main" has no game object "/gaem"`,
		},
		"three parts, hashed": {
			script: `function init(self) self.game = msg.url("main", hash("/game"), hash("gaem")) end`,
			want:   `"main:/game#gaem": "/game" has no component "gaem"`,
		},
		"kept in a constant": {
			script: `local SPRITE = msg.url("#sprit")
function init(self) self.sprite = SPRITE end`,
			want: `"#sprit": "/ball" has no component "sprit"`,
		},
		"default of a property": {
			script: `go.property("target", msg.url("/gaem"))`,
			want:   `collection "main" has no game object "/gaem"`,
		},
	}
	for name, testCase := range built {
		t.Run(name, func(t *testing.T) {
			problems := problemsIn(t, pongWithBallScript(testCase.script+"\n"))

			assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 1, testCase.want)
		})
	}
}

func TestHashOfAGameObjectThatDoesNotExist(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function update(self, dt)
    go.set_position(vmath.vector3(), hash("/gaem"))
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 2,
		`"/gaem": collection "main" has no game object "/gaem"`, `did you mean "/game"?`)
}

func TestAddressesThatAreNotWrittenDownAreLeftAlone(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithBallScript(`local NAME = "/nowhere"
local target = "/nowhere"
target = "/game"
local built = "/no" .. "where"

function init(self)
    print(NAME)
    self.me = msg.url()
    self.other = msg.url(some_path)
    self.part = msg.url(nil, go.get_id("nowhere"), "script")
    self.ids = { hash("/nowhere") }
    go.set_position(vmath.vector3(), target)
    go.set_position(vmath.vector3(), built)
    -- The id of a game object is the hash of its whole path. A hash of anything
    -- else is not an address defold-vet can follow.
    go.set_position(vmath.vector3(), hash("nowhere"))
end
`)))
}

const probeInALevel = `name: "level"
instances {
  id: "probe"
  prototype: "/level/probe.go"
}
`

const probeWithScript = `components {
  id: "probe"
  component: "/level/probe.script"
}
`

func pongWithALevel(probeScript string) map[string]string {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["level/level.collection"] = probeInALevel
	files["level/probe.go"] = probeWithScript
	files["level/probe.script"] = probeScript
	return files
}

// A collection loaded through a proxy is addressed by its name. Every collection
// of the project that is not nested in another can be one.
func TestAddressInAnotherCollectionOfTheProject(t *testing.T) {
	problems := problemsIn(t, pongWithALevel(`function init(self)
    msg.post("main:/game#game", "enable")
    msg.post("main:/game#gaem", "enable")
    msg.post("main:/gaem", "enable")
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unresolvedURLCheck, "level/probe.script", 3, `"/game" has no component "gaem"`)
	assertOneProblem(t, problems[1:], unresolvedURLCheck, "level/probe.script", 4, `collection "main" has no game object "/gaem"`, `did you mean "/game"?`)
}

func TestAddressesInCollectionsThatCannotBeFollowedAreLeftAlone(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithALevel(`function init(self)
    msg.post("@render:", "clear_color", { color = vmath.vector4() })
    msg.post("menu:/nowhere#nothing", "enable")
    msg.post("main:game", "enable")
end
`)))
}

func TestAnAddressThatIsRightInOneOfTheCollectionsOfThatNameIsNotReported(t *testing.T) {
	files := pongWithALevel(`function init(self)
    msg.post("level:/boss", "enable")
    msg.post("level:/nobody", "enable")
end
`)
	files["level/second.collection"] = `name: "level"
instances {
  id: "boss"
  prototype: "/level/probe.go"
}
`

	assertOneProblem(t, problemsIn(t, files), unresolvedURLCheck, "level/probe.script", 3, `"level:/nobody"`)
}

func TestMessagePostedToAnAddressKeptInAConstant(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`local GAME = msg.url("/game#game")

function update(self, dt)
    msg.post(GAME, "ball_out")
    msg.post(GAME, "bal_out")
end
`, gameHandlingBallOut))

	assertOneProblem(t, problemsOf(problems, unhandledMessageCheck), unhandledMessageCheck, "main/ball.script", 5,
		`"bal_out" is posted to "/game#game"`, `/main/game.script does not handle it`)
}

func TestMessagePostedToAScriptInAnotherCollection(t *testing.T) {
	files := pongWithALevel(`function final(self)
    msg.post("main:/game#game", "ball_out")
    msg.post("main:/game#game", "level_done")
end
`)
	files["main/game.script"] = gameHandlingBallOut
	problems := problemsIn(t, files)

	assertOneProblem(t, problems, unhandledMessageCheck, "level/probe.script", 3,
		`"level_done" is posted to "main:/game#game"`, `/main/game.script does not handle it`)
}

// A collection that comes from a library is not there to be read. What it holds
// lives under the id it was given, and nothing can be said about those paths.
func TestAddressUnderACollectionThatCouldNotBeReadIsLeftAlone(t *testing.T) {
	files := pongWithBallScript(`function init(self)
    msg.post("/hud/score#gui", "enable")
    msg.post("/hud", "enable")
    msg.post("/huds/score", "enable")
end
`)
	files["main/main.collection"] += `collection_instances {
  id: "hud"
  collection: "/library/hud.collection"
}
`

	problems := problemsOf(problemsIn(t, files), unresolvedURLCheck)

	if len(problems) != 2 || problems[0].Location.Position.Line != 3 || problems[1].Location.Position.Line != 4 {
		t.Errorf("problems = %+v, want the two addresses that are not under /hud/", problems)
	}
}

const boatSpawner = `components {
  id: "boats"
  component: "/main/boats.collectionfactory"
}
`

// A collection spawned by a collection factory lands in the collection that
// spawns it. Its absolute addresses belong to that one, which is not known; the
// relative ones stay inside it.
func TestACollectionAFactorySpawnsIsJudgedOnlyInsideItself(t *testing.T) {
	files := pongWithALevel(`function init(self)
    msg.post("/camera", "follow")
    msg.post("level:/camera", "follow")
    msg.post("probe#probe", "enable")
    msg.post("prob#probe", "enable")
end
`)
	files["main/spawner.go"] = boatSpawner
	files["main/boats.collectionfactory"] = "prototype: \"/level/level.collection\"\n"

	assertOneProblem(t, problemsOf(problemsIn(t, files), unresolvedURLCheck), unresolvedURLCheck, "level/probe.script", 5,
		`"prob#probe"`, `did you mean "/probe"?`)
}

// A game object a factory spawns may also sit in a collection. Where the factory
// puts it is one more place its script runs, and that place is not known.
func TestAGameObjectAFactoryAlsoSpawnsIsNotJudgedByWhereItSits(t *testing.T) {
	files := pongWithBallScript(`function init(self)
    msg.post("/arena", "ball_spawned")
    msg.post("#sprit", "enable")
end
`)
	files["main/spawner.go"] = `embedded_components {
  id: "balls"
  type: "factory"
  data: "prototype: \"/main/ball.go\"\n"
}
`

	assertOneProblem(t, problemsIn(t, files), unresolvedURLCheck, "main/ball.script", 3, `"#sprit"`)
}

func TestAnAddressTheScriptAsksAboutIsNotAssumedToExist(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function init(self)
    if go.exists("/frame") then
        go.set_parent(".", "/frame", true)
    end
    go.set_parent(".", "/nowhere", true)
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 5, `"/nowhere"`)
}

// The reference lists a URL among the types of a mesh id. A mesh is not addressed.
func TestTheIDOfAMeshIsNotAnAddress(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithBallScript(`function init(self)
    model.set_mesh_enabled("#sprite", "Knight_Helmet", false)
    self.visible = model.get_mesh_enabled("#sprite", "Knight_Cape")
end
`)))
}

func TestAPathPatternIsNotAFile(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/custom.texture_profiles"] = `path_settings {
  path: "/main/**/*.atlas"
  profile: "Default"
}
path_settings {
  path: "/main/missing.atlas"
  profile: "Default"
}
`

	assertOneProblem(t, problemsIn(t, files), missingResourceCheck, "main/custom.texture_profiles", 6, `"/main/missing.atlas"`)
}

func TestALuaModuleThatPostsToAnotherSocket(t *testing.T) {
	files := pongWithScripts("", gameHandlingBallOut)
	files["main/debug.lua"] = `local M = {}

function M.draw(game)
    msg.post("@render:", "draw_overlay")
    msg.post("main:/game#game", "ball_out")
    msg.post("main:/game#game", "ball_in")
end

return M
`

	assertOneProblem(t, problemsIn(t, files), unhandledMessageCheck, "main/debug.lua", 6,
		`"ball_in" is posted to "main:/game#game"`, `/main/game.script does not handle it`)
}

// What a library posts is for the project that uses it to handle.
func TestALibraryIsNotAskedWhoHandlesWhatItPosts(t *testing.T) {
	files := pongWithScripts(`local function notify(listener)
    msg.post(listener, "window_update")
end
`, "")
	files["game.project"] = "[library]\ninclude_dirs = main\n"

	assertNoProblems(t, problemsIn(t, files))
}
