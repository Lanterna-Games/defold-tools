package check

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

func problemsIn(t *testing.T, files map[string]string) []Problem {
	t.Helper()
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	problems := Run(loaded, reference, Options{TurnedOff: map[string]bool{unverifiableCheck: true, unusedFileCheck: true, unsetSettingCheck: true}})
	for _, problem := range problems {
		if saysWhatWasMeant := strings.Contains(problem.Message, "did you mean"); saysWhatWasMeant != (problem.Suggestion != "") {
			t.Errorf("the message %q and the suggestion %q do not agree: a fix is offered from the suggestion", problem.Message, problem.Suggestion)
		}
	}
	return problems
}

func assertNoProblems(t *testing.T, problems []Problem) {
	t.Helper()
	for _, problem := range problems {
		t.Errorf("unexpected problem at %s:%d: %s [%s]", problem.Location.File, problem.Location.Position.Line, problem.Message, problem.Check)
	}
}

func assertOneProblem(t *testing.T, problems []Problem, check, file string, line int, messageParts ...string) {
	t.Helper()
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %+v", len(problems), problems)
	}
	problem := problems[0]
	if problem.Check != check {
		t.Errorf("check = %q, want %q", problem.Check, check)
	}
	if problem.Location.File != file || problem.Location.Position.Line != line {
		t.Errorf("reported at %s:%d, want %s:%d", problem.Location.File, problem.Location.Position.Line, file, line)
	}
	for _, part := range messageParts {
		if !strings.Contains(problem.Message, part) {
			t.Errorf("message = %q, want it to contain %q", problem.Message, part)
		}
	}
}

const ballWithSpriteAndScript = `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"ball\"\n"
  ""
}
`

const gameWithScript = `components {
  id: "game"
  component: "/main/game.script"
}
`

func mainCollectionPointingBallAt(url string) string {
	return `name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  component_properties {
    id: "ball"
    properties {
      id: "game"
      value: "` + url + `"
      type: PROPERTY_TYPE_URL
    }
  }
}
instances {
  id: "game"
  prototype: "/main/game.go"
}
`
}

const ballScriptDeclaringGame = `go.property("game", msg.url())
`

func pongWith(mainCollection string) map[string]string {
	return map[string]string{
		"main/main.collection": mainCollection,
		"main/ball.go":         ballWithSpriteAndScript,
		"main/game.go":         gameWithScript,
		"main/ball.script":     ballScriptDeclaringGame,
		"main/game.script":     "",
	}
}

func TestAValidProjectHasNoProblems(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWith(mainCollectionPointingBallAt("/game#game"))))
}

func TestURLPropertyPointingAtAGameObjectThatDoesNotExist(t *testing.T) {
	problems := problemsIn(t, pongWith(mainCollectionPointingBallAt("/gme#game")))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/main.collection", 9,
		`property "game" of "/ball#ball"`, `collection "main" has no game object "/gme"`, `did you mean "/game"?`)
}

func TestURLPropertyPointingAtAComponentThatDoesNotExist(t *testing.T) {
	problems := problemsIn(t, pongWith(mainCollectionPointingBallAt("/game#gaem")))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/main.collection", 9,
		`"/game" has no component "gaem"`, `did you mean "game"?`)
}

func TestURLPropertyForms(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "another game object, without a component", url: "/game"},
		{name: "relative to the collection", url: "game#game"},
		{name: "a component of the same game object", url: "#sprite"},
		{name: "the game object itself", url: "."},
		{name: "with the socket of this collection", url: "main:/game#game"},
		// Another socket is another collection, loaded at run time: it cannot be
		// judged from here.
		{name: "with the socket of another collection", url: "level:/door#door"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNoProblems(t, problemsIn(t, pongWith(mainCollectionPointingBallAt(tt.url))))
		})
	}
}

func TestURLPropertyPointingAtAComponentOfTheSameGameObjectThatDoesNotExist(t *testing.T) {
	problems := problemsIn(t, pongWith(mainCollectionPointingBallAt("#sprit")))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/main.collection", 9,
		`"/ball" has no component "sprit"`, `did you mean "sprite"?`)
}

// An absolute URL is resolved from the root collection at run time, not from
// the file it is written in. Written in a nested collection, "/game" means the
// game object of whichever collection nests it.
func TestAbsoluteURLInANestedCollectionIsResolvedFromItsRoot(t *testing.T) {
	files := pongWith(`name: "main"
instances {
  id: "game"
  prototype: "/main/game.go"
}
collection_instances {
  id: "level"
  collection: "/main/level.collection"
}
`)
	files["main/level.collection"] = `name: "level"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  component_properties {
    id: "ball"
    properties {
      id: "game"
      value: "/game#game"
      type: PROPERTY_TYPE_URL
    }
  }
}
`

	assertNoProblems(t, problemsIn(t, files))
}

func TestRelativeURLInANestedCollectionStaysInsideIt(t *testing.T) {
	files := pongWith(`name: "main"
instances {
  id: "game"
  prototype: "/main/game.go"
}
collection_instances {
  id: "level"
  collection: "/main/level.collection"
}
`)
	files["main/level.collection"] = `name: "level"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  component_properties {
    id: "ball"
    properties {
      id: "game"
      value: "game#game"
      type: PROPERTY_TYPE_URL
    }
  }
}
`

	assertOneProblem(t, problemsIn(t, files), unresolvedURLCheck, "main/level.collection", 9,
		`property "game" of "/level/ball#ball"`, `no game object "/level/game"`)
}

// A collection nested in two roots is right if it is right in one of them:
// reporting it because of the other would be a false positive.
func TestAURLThatResolvesInOneOfTheRootsIsNotReported(t *testing.T) {
	files := map[string]string{
		"main/with_game.collection": `name: "with_game"
instances {
  id: "game"
  prototype: "/main/game.go"
}
collection_instances {
  id: "level"
  collection: "/main/level.collection"
}
`,
		"main/without_game.collection": `name: "without_game"
collection_instances {
  id: "level"
  collection: "/main/level.collection"
}
`,
		"main/level.collection": `name: "level"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  component_properties {
    id: "ball"
    properties {
      id: "game"
      value: "/game#game"
      type: PROPERTY_TYPE_URL
    }
  }
}
`,
		"main/ball.go":     ballWithSpriteAndScript,
		"main/game.go":     gameWithScript,
		"main/ball.script": ballScriptDeclaringGame,
		"main/game.script": "",
	}

	assertNoProblems(t, problemsIn(t, files))
}

func TestPropertiesThatAreNotURLsAreLeftAlone(t *testing.T) {
	collection := strings.Replace(mainCollectionPointingBallAt("/nowhere"), "PROPERTY_TYPE_URL", "PROPERTY_TYPE_HASH", 1)

	assertNoProblems(t, problemsIn(t, pongWith(collection)))
}

func TestMissingResource(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	delete(files, "main/ball.script")
	files["main/bal.script"] = ""

	assertOneProblem(t, problemsIn(t, files), missingResourceCheck, "main/ball.go", 3,
		`"/main/ball.script" does not exist`, `did you mean "/main/bal.script"?`)
}

func TestMissingResourceInsideAnEmbeddedComponent(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/ball.go"] = `embedded_components {
  id: "sprite"
  type: "sprite"
  data: "textures {\n"
  "  texture: \"/main/pong.atlas\"\n"
  "}\n"
  ""
}
`

	assertOneProblem(t, problemsIn(t, files), missingResourceCheck, "main/ball.go", 5, `"/main/pong.atlas" does not exist`)
}

// Files under a directory the project does not have come from the engine
// (/builtins) or from a library dependency, which is not unpacked on disk.
func TestResourcesOutsideTheProjectTreeAreNotJudged(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/ball.go"] = `embedded_components {
  id: "sprite"
  type: "sprite"
  data: "material: \"/builtins/materials/sprite.material\"\n"
  "textures {\n"
  "  texture: \"/some_library/shared.atlas\"\n"
  "}\n"
  ""
}
`

	assertNoProblems(t, problemsIn(t, files))
}

func TestUnknownChild(t *testing.T) {
	files := pongWith(`name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  children: "gaem"
}
instances {
  id: "game"
  prototype: "/main/game.go"
}
`)

	assertOneProblem(t, problemsIn(t, files), unknownChildCheck, "main/main.collection", 5,
		`"ball" lists the child "gaem"`, `did you mean "game"?`)
}

func TestSyntaxErrorIsReportedAndTheRestIsStillChecked(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/gme#game"))
	files["main/game.go"] = "components {\n  id: \"game\"\n"

	problems := problemsIn(t, files)

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want the syntax error and the broken URL: %+v", len(problems), problems)
	}
	if problems[0].Check != syntaxCheck || problems[0].Location.File != "main/game.go" {
		t.Errorf("first problem = %+v, want the syntax error in game.go", problems[0])
	}
	if problems[1].Check != unresolvedURLCheck {
		t.Errorf("second problem = %+v, want the unresolved URL", problems[1])
	}
}

func TestProblemsAreOrderedByFileAndPosition(t *testing.T) {
	files := pongWith(`name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  children: "nobody"
  component_properties {
    id: "ball"
    properties {
      id: "game"
      value: "/nowhere"
      type: PROPERTY_TYPE_URL
    }
  }
}
`)
	delete(files, "main/ball.script")

	problems := problemsIn(t, files)

	var order []string
	for _, problem := range problems {
		order = append(order, problem.Location.File+":"+problem.Check)
	}
	want := "main/ball.go:missing-resource main/main.collection:unknown-child main/main.collection:unresolved-url"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order = %q, want %q", got, want)
	}
}

func pongWithBallScript(script string) map[string]string {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/ball.script"] = script + ballScriptDeclaringGame
	return files
}

func TestScriptURLPointingAtAComponentThatDoesNotExist(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function init(self)
    msg.post("#sprit", "disable")
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 2,
		`"#sprit": "/ball" has no component "sprit"`, `did you mean "sprite"?`)
}

func TestScriptURLPointingAtAGameObjectThatDoesNotExist(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function init(self)
    go.set_position(vmath.vector3(), "/gaem")
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 2,
		`"/gaem": collection "main" has no game object "/gaem"`, `did you mean "/game"?`)
}

func TestScriptURLsThatResolve(t *testing.T) {
	files := pongWithBallScript(`local OTHER = "/no" .. "where"

function init(self)
    self.target, self.index, self.label = msg.url(), 1, {}
    msg.post(".", "acquire_input_focus")
    msg.post("#", "ready")
    msg.post("#sprite", "disable")
    msg.post("/game#game", "ball_out")
    msg.post("game", "ball_out")
    msg.post("main:/game", "ball_out")
    sprite.play_flipbook("#sprite", "roll")
    go.set_position(vmath.vector3(), "/game")
end

function update(self, dt)
    -- What is not written as a literal cannot be judged, and is left alone.
    msg.post(self.target, "tick")
    msg.post(OTHER, "tick")
    msg.post("/enemy" .. self.index, "tick")
    -- A string is only an address where the engine expects one.
    print("#nothing")
    go.set("#sprite", "tint", "#not_a_url")
    self.label:set("#nothing")
end

function on_message(self, message_id)
    if message_id == hash("ready") then
        self.ready = true
    end
end
`)
	files["main/game.script"] = `function on_message(self, message_id)
    if message_id == hash("ball_out") or message_id == hash("tick") then
        self.busy = true
    end
end
`

	assertNoProblems(t, problemsIn(t, files))
}

// The same script on two game objects is right if it is right on one of them.
func TestAScriptSharedByGameObjectsIsJudgedAgainstAllOfThem(t *testing.T) {
	files := pongWithBallScript(`function init(self)
    msg.post("#sprite", "disable")
    msg.post("#nowhere", "disable")
end
`)
	files["main/game.go"] = `components {
  id: "game"
  component: "/main/ball.script"
}
`

	assertOneProblem(t, problemsIn(t, files), unresolvedURLCheck, "main/ball.script", 3, `"#nowhere"`)
}

// A game object that only a factory spawns sits in no collection. Its own
// components are known; where it will live is not.
func TestScriptOfAGameObjectThatIsInNoCollection(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/bullet.go"] = `components {
  id: "bullet"
  component: "/main/bullet.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: ""
}
`
	files["main/bullet.script"] = `function init(self)
    msg.post("#sprite", "enable")
    msg.post("#trail", "enable")
    msg.post("/game", "bullet_fired")
    msg.post("/nowhere", "bullet_fired")
end
`

	assertOneProblem(t, problemsIn(t, files), unresolvedURLCheck, "main/bullet.script", 3,
		`"#trail": "/main/bullet.go" has no component "trail"`)
}

func TestAScriptNoGameObjectUsesIsNotJudged(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/unused.script"] = `msg.post("#nowhere", "disable")`

	assertNoProblems(t, problemsIn(t, files))
}

// Lua syntax is the Lua language server's job. A script that does not parse is
// checked as far as it could be read, and its syntax errors are not repeated.
func TestBrokenLuaIsCheckedAsFarAsItCouldBeRead(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function init(self)
    msg.post("#sprit", "disable")
    self.speed =
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 2, `"#sprit"`)
}
