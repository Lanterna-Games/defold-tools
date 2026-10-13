package check

import (
	"strings"
	"testing"
)

func TestCollectionSetsAPropertyTheScriptDoesNotDeclare(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/ball.script"] = `go.property("speed", 320)
go.property("target", msg.url())
`

	assertOneProblem(t, problemsIn(t, files), undeclaredPropertyCheck, "main/main.collection", 8,
		`"/ball#ball" sets the property "game"`, `/main/ball.script does not declare it`, `it declares "speed" and "target"`)
}

func TestCollectionSetsAPropertyWhoseNameIsAlmostRight(t *testing.T) {
	files := pongWith(strings.Replace(mainCollectionPointingBallAt("/game#game"), `id: "game"`, `id: "gaem"`, 1))

	assertOneProblem(t, problemsIn(t, files), undeclaredPropertyCheck, "main/main.collection", 8,
		`sets the property "gaem"`, `did you mean "game"?`)
}

func TestPropertiesThatCannotBeJudgedAreLeftAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		// A name built at run time could be anything, "game" included.
		{name: "a property declared with a computed name", script: `go.property(PREFIX .. "game", msg.url())`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := pongWith(mainCollectionPointingBallAt("/game#game"))
			files["main/ball.script"] = tt.script

			assertNoProblems(t, problemsIn(t, files))
		})
	}
}

func TestPropertyOfAScriptThatIsNotInTheProjectIsNotJudged(t *testing.T) {
	files := pongWith(mainCollectionPointingBallAt("/game#game"))
	files["main/ball.go"] = strings.Replace(ballWithSpriteAndScript, "/main/ball.script", "/some_library/ball.script", 1)

	assertNoProblems(t, problemsIn(t, files))
}

func scriptProblems(t *testing.T, script string) []Problem {
	t.Helper()
	return problemsIn(t, pongWithBallScript(script))
}

func TestSelfFieldReadButNeverAssigned(t *testing.T) {
	problems := scriptProblems(t, `function init(self)
    self.velocity = vmath.vector3()
end

function update(self, dt)
    go.set_position(go.get_position() + self.velocty * dt)
end
`)

	assertOneProblem(t, problems, unassignedSelfFieldCheck, "main/ball.script", 6,
		"self.velocty is read, but this script never assigns it", "did you mean self.velocity?")
}

func TestSelfFieldsThatAreAssignedSomewhere(t *testing.T) {
	assertNoProblems(t, scriptProblems(t, `go.property("speed", 320)

local function reset(self)
    self.direction = vmath.vector3(1, 0, 0)
end

function init(self)
    reset(self)
    self.elapsed, self.hits = 0, 0
    timer.delay(1, true, function(self, handle, time_elapsed)
        self.ticks = (self.ticks or 0) + 1
    end)
end

function update(self, dt)
    self.elapsed = self.elapsed + dt
    print(self.speed, self.game, self.direction, self.hits, self.ticks)
end
`))
}

// A script may define a class of its own. Inside its methods "self" is an
// object of that class, not the script instance, and its fields are none of
// this check's business.
func TestSelfInsideAMethodIsNotTheScriptInstance(t *testing.T) {
	assertNoProblems(t, scriptProblems(t, `local Hazard = {}
Hazard.__index = Hazard

function Hazard:check(player)
    return self.x + self.box_x, player.y
end

function Hazard.touches(self, other)
    return self.hw > other.hw
end

function init(self)
    world.add(setmetatable({ box_x = 1 }, Hazard))
end
`))
}

func TestHelperThatReceivesTheInstanceIsFollowed(t *testing.T) {
	problems := scriptProblems(t, `local function bounce(self)
    self.speed = self.sped * -1
end

function update(self, dt)
    bounce(self)
end
`)

	assertOneProblem(t, problems, unassignedSelfFieldCheck, "main/ball.script", 2, "self.sped is read")
}

// A helper nobody calls with the instance may be about anything.
func TestHelperThatIsNeverGivenTheInstanceIsNotJudged(t *testing.T) {
	assertNoProblems(t, scriptProblems(t, `local function describe(self)
    return self.name
end

function init(self)
    print(describe({ name = "ball" }))
end
`))
}

// Once the instance is handed to code this script does not contain, any field
// may be assigned there.
func TestScriptsThatLetTheInstanceOutOfSightAreNotJudged(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{name: "passed to a module", script: "local setup = require \"main.setup\"\nfunction init(self)\n    setup.prepare(self)\n    print(self.ready)\nend\n"},
		{name: "kept in another variable", script: "function init(self)\n    local ball = self\n    ball.ready = true\n    print(self.ready)\nend\n"},
		{name: "indexed with a computed name", script: "function init(self)\n    self[\"rea\" .. \"dy\"] = true\n    print(self.ready)\nend\n"},
		{name: "stored in a table", script: "function init(self)\n    registry.add({ owner = self })\n    print(self.ready)\nend\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNoProblems(t, scriptProblems(t, tt.script))
		})
	}
}
