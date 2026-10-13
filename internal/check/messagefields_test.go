package check

import "testing"

func TestFieldsOfWhatTheEngineSends(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithBallScript(`local CONTACT = hash("contact_point_response")

function on_message(self, message_id, message, sender)
    if message_id == hash("collision_response") then
        self.hit = message.other_id
        self.at = message.other_position.x
        self.old = message.group
    elseif message_id == CONTACT and message.distance > 0 then
        self.normal = message.normal
    elseif message_id == hash("trigger_response") or message_id == hash("animation_done") then
        self.entered = message.enter
        self.tile = message.current_tile
    end
end
`)))
}

func TestFieldTheEngineDoesNotSend(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function on_message(self, message_id, message, sender)
    if message_id == hash("collision_response") then
        self.hit = message.other_grup
    elseif message_id == hash("trigger_response") then
        if message.enter then
            self.inside = true
        elseif message.exit then
            self.inside = false
        end
    end
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownMessageFieldCheck, "main/ball.script", 3,
		`"other_grup" is not a field of "collision_response"`, `did you mean "other_group"?`)
	assertOneProblem(t, problems[1:], unknownMessageFieldCheck, "main/ball.script", 7,
		`"exit" is not a field of "trigger_response"`)
}

func TestFieldsThatAPostSets(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithScripts(`local SCORE = { side = 1, points = 2 }

function update(self, dt)
    msg.post("/game#game", "ball_out", SCORE)
    msg.post("/game#game", "ball_out", { side = 2, ["bonus"] = true })
    msg.post("/game#game", "reset")
end
`, `function on_message(self, message_id, message, sender)
    if message_id == hash("ball_out") then
        self.score = message.side + message.points
        self.bonus = message.bonus
    elseif message_id == hash("reset") then
        self.score = 0
    end
end
`)))
}

func TestFieldNoPostSets(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "ball_out", { side = 1 })
    msg.post("/game#game", "reset")
end
`, `function on_message(self, message_id, message, sender)
    if message_id == hash("ball_out") then
        self.score = message.sid
    elseif message_id == hash("reset") then
        self.score = message.score
    end
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownMessageFieldCheck, "main/game.script", 3,
		`"sid" is read from "ball_out", but nothing that posts it sets "sid"`, `did you mean "side"?`)
	assertOneProblem(t, problems[1:], unknownMessageFieldCheck, "main/game.script", 5, `"score" is read from "reset"`)
}

func TestTheMessageAHandlerIsInIsFollowedThroughItsBranches(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "ball_out", { side = 1 })
    msg.post("/game#game", "serve", { speed = 2 })
end
`, `local BALL_OUT = hash("ball_out")

function on_message(self, message_id, message, sender)
    self.anything = message.anything
    if message_id ~= BALL_OUT and message_id ~= hash("serve") then
        return
    end
    self.either = message.side or message.speed
    self.neither = message.neither
    if message_id == BALL_OUT then
        for index = 1, 3 do
            self.side = message.side + index
            self.speed = message.speed
        end
    end
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownMessageFieldCheck, "main/game.script", 9,
		`"neither" is read from any of "ball_out" and "serve"`)
	assertOneProblem(t, problems[1:], unknownMessageFieldCheck, "main/game.script", 13, `"speed" is read from "ball_out"`)
}

func TestFieldsOfAMessageThatCannotAllBeKnownAreLeftAlone(t *testing.T) {
	handler := `function on_message(self, message_id, message, sender)
    if message_id == hash("ball_out") then
        self.score = message.anything
    end
end
`
	posts := map[string]string{
		"a table that is built elsewhere":       `msg.post("/game#game", "ball_out", self.payload)`,
		"a table with a key that is not a name": `msg.post("/game#game", "ball_out", { [self.key] = 1 })`,
		"a post whose id cannot be read":        `msg.post("/game#game", "ball_out", { side = 1 }) msg.post("/game#game", self.id, { anything = 1 })`,
		"no post at all":                        `self.idle = true`,
	}
	for name, post := range posts {
		t.Run(name, func(t *testing.T) {
			problems := problemsIn(t, pongWithScripts("function update(self, dt)\n    "+post+"\nend\n", handler))

			assertNoProblems(t, problemsOf(problems, unknownMessageFieldCheck))
		})
	}
	t.Run("a library may post it", func(t *testing.T) {
		files := pongWithScripts(`function update(self, dt) msg.post("/game#game", "ball_out", { side = 1 }) end`+"\n", handler)
		files["game.project"] = settingsWithALibrary

		assertNoProblems(t, problemsOf(problemsIn(t, files), unknownMessageFieldCheck))
	})
}

// "load" is a message of the engine, and nothing stops a project from posting
// one of its own under the same name, with fields of its own.
func TestAMessageTheProjectPostsUnderAnEngineNameHasTheFieldsThePostSets(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "load", { level = 3 })
end
`, `function on_message(self, message_id, message, sender)
    if message_id == hash("load") then
        self.level = message.level
        self.mode = message.mode
    end
end
`))

	assertOneProblem(t, problems, unknownMessageFieldCheck, "main/game.script", 4, `"mode" is read from "load"`)
}

func TestAFieldTheHandlerSetsItselfIsNotExpectedFromThePost(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "ball_out", { side = 1 })
end
`, `function on_message(self, message_id, message, sender)
    if message_id == hash("ball_out") then
        message.points = message.points or 1
        self.score = message.points + message.side
    end
end
`)))
}
