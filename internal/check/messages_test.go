package check

import (
	"maps"
	"strings"
	"testing"
)

const gameHandlingBallOut = `local BALL_OUT = hash("ball_out")

function on_message(self, message_id, message, sender)
    if message_id == BALL_OUT then
        self.score = 1
    end
end
`

func pongWithScripts(ballScript, gameScript string) map[string]string {
	files := pongWithBallScript(ballScript)
	files["main/game.script"] = gameScript
	return files
}

func problemsOf(problems []Problem, check string) []Problem {
	var selected []Problem
	for _, problem := range problems {
		if problem.Check == check {
			selected = append(selected, problem)
		}
	}
	return selected
}

func TestMessagePostedToAScriptThatHandlesIt(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "ball_out", { side = 1 })
    msg.post("/game", hash("ball_out"))
end
`, gameHandlingBallOut)))
}

// One typo is reported from both ends: the post nobody handles, and the handler
// nobody posts to.
func TestMessagePostedToAScriptThatDoesNotHandleIt(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "bal_out")
end
`, gameHandlingBallOut))

	assertOneProblem(t, problemsOf(problems, unhandledMessageCheck), unhandledMessageCheck, "main/ball.script", 2,
		`"bal_out" is posted to "/game#game"`, `/main/game.script does not handle it`, `did you mean "ball_out"?`)
	assertOneProblem(t, problemsOf(problems, unpostedMessageCheck), unpostedMessageCheck, "main/game.script", 4,
		`"ball_out" is handled here, but nothing in the project posts it`, `did you mean "bal_out"?`)
}

func TestMessageIDKeptInAConstantIsReportedWhereItIsPosted(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`local PADDLE_HIT = hash("paddle_hit")

function update(self, dt)
    msg.post("/game#game", PADDLE_HIT)
end
`, gameHandlingBallOut))

	assertOneProblem(t, problemsOf(problems, unhandledMessageCheck), unhandledMessageCheck, "main/ball.script", 4,
		`"paddle_hit" is posted to "/game#game"`, `/main/game.script does not handle it; it handles "ball_out"`)
}

func TestMessagePostedToAGameObjectNoneOfWhoseComponentsHandlesIt(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game", "explode")
end
`, ""))

	assertOneProblem(t, problems, unhandledMessageCheck, "main/ball.script", 2,
		`"explode" is posted to "/game"`, `/main/game.script does not handle it`)
}

func TestMessageOfTheProjectPostedToAComponentThatIsNotAScript(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("#sprite", "explode")
    msg.post("#sprite", "disable")
end
`, ""))

	assertOneProblem(t, problems, unhandledMessageCheck, "main/ball.script", 2,
		`"explode" is posted to "#sprite"`, `it is a sprite component, which takes engine messages only`)
}

func TestMessagePostedToTheScriptItself(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function init(self)
    msg.post("#", "serve")
    msg.post("#", "sreve")
end

function on_message(self, message_id)
    if message_id == hash("serve") then
        self.served = true
    end
end
`, ""))

	assertOneProblem(t, problems, unhandledMessageCheck, "main/ball.script", 3,
		`/main/ball.script does not handle it`, `did you mean "serve"?`)
}

// What a handler does with a message it does not name cannot be known, so a
// script that lets the id out of sight is taken to handle everything.
func TestHandlersThatCouldHandleAnyMessageAreLeftAlone(t *testing.T) {
	handlers := map[string]string{
		"passes the id on": `function on_message(self, message_id, message)
    dispatch(message_id, message)
end`,
		"looks the id up in a table": `local handlers = {}
function on_message(self, message_id, message)
    local handle = handlers[message_id]
    if handle then handle(self, message) end
end`,
		"compares the id with something that is not written here": `local ids = require("main.ids")
function on_message(self, message_id)
    if message_id == ids.BALL_OUT then self.out = true end
end`,
		"treats every other message in an else": `function on_message(self, message_id)
    if message_id == hash("ball_out") then
        self.out = true
    else
        self.other = true
    end
end`,
		"acts on everything except one message": `function on_message(self, message_id)
    if message_id ~= hash("ball_out") then
        self.other = true
    end
end`,
		"never looks at the id": `function on_message(self)
    go.delete()
end`,
		"is assigned and not declared": `on_message = require("main.handler")`,
		"did not parse": `function on_message(self, message_id)
    if message_id == hash("ball_out") then
        self.out =
    end
end`,
	}
	for name, gameScript := range handlers {
		t.Run(name, func(t *testing.T) {
			problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "explode")
    msg.post(self.game, "implode")
end
`, gameScript))

			assertNoProblems(t, problemsOf(problems, unhandledMessageCheck))
		})
	}
}

func TestHandlerThatReturnsEarlyOnEveryOtherMessage(t *testing.T) {
	gameScript := `local BALL_OUT = hash("ball_out")

function on_message(self, message_id, message)
    if message_id ~= BALL_OUT or not message.side then
        return
    end
    self.score = message.side
end
`
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "ball_out", { side = 1 })
    msg.post("/game#game", "ball_in")
end
`, gameScript))

	assertOneProblem(t, problems, unhandledMessageCheck, "main/ball.script", 3, `"ball_in"`)
}

// The handler of a new script is the empty one the editor's template leaves.
func TestAnEmptyHandlerHandlesNothing(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "explode")
end
`, `function on_message(self, message_id, message, sender)
    -- Add message-handling code here
end
`))

	assertOneProblem(t, problems, unhandledMessageCheck, "main/ball.script", 2, `/main/game.script does not handle it`)
}

func TestReceiversThatCannotBeFollowedAreLeftAlone(t *testing.T) {
	files := pongWithScripts(`function update(self, dt)
    msg.post("@render:", "set_zoom")
    msg.post("other:/game#game", "explode")
    msg.post("/game#gaem", "ball_out")
    msg.post("/gaem", "ball_out")
end
`, gameHandlingBallOut)

	assertNoProblems(t, problemsOf(problemsIn(t, files), unhandledMessageCheck))
}

func TestMessagePostedToAGUIGoesToTheScriptOfItsScene(t *testing.T) {
	files := pongWithScripts(`function update(self, dt)
    msg.post("/game#hud", "set_score")
    msg.post("/game#hud", "set_scroe")
end
`, "")
	files["main/game.go"] = gameWithScript + `components {
  id: "hud"
  component: "/main/hud.gui"
}
`
	files["main/hud.gui"] = `script: "/main/hud.gui_script"
nodes {
  id: "score"
  type: TYPE_TEXT
}
`
	files["main/hud.gui_script"] = `function on_message(self, message_id, message)
    if message_id == hash("set_score") then
        gui.set_text(gui.get_node("score"), "1")
    end
end
`

	assertOneProblem(t, problemsIn(t, files), unhandledMessageCheck, "main/ball.script", 3,
		`/main/hud.gui_script does not handle it`, `did you mean "set_score"?`)
}

func TestAGUIWhoseSceneIsNotInTheProjectIsNotJudged(t *testing.T) {
	files := pongWithScripts(`function update(self, dt)
    msg.post("/game#hud", "set_score")
end
`, "")
	files["main/game.go"] = gameWithScript + `components {
  id: "hud"
  component: "/library/hud.gui"
}
`

	assertNoProblems(t, problemsOf(problemsIn(t, files), unhandledMessageCheck))
}

// The same script on two game objects is right if it is right on one of them.
func TestAPostFromASharedScriptIsJudgedAgainstEveryGameObjectThatUsesIt(t *testing.T) {
	files := pongWithScripts(`function update(self, dt)
    msg.post("#listener", "tick")
    msg.post("#listener", "tock")
end
`, "")
	files["main/ball.go"] = ballWithSpriteAndScript + `components {
  id: "listener"
  component: "/main/deaf.script"
}
`
	files["main/game.go"] = `components {
  id: "game"
  component: "/main/ball.script"
}
components {
  id: "listener"
  component: "/main/listener.script"
}
`
	files["main/deaf.script"] = ""
	files["main/listener.script"] = `function on_message(self, message_id)
    if message_id == hash("tick") then
        self.ticks = 1
    end
end
`

	assertOneProblem(t, problemsIn(t, files), unhandledMessageCheck, "main/ball.script", 3, `"tock"`)
}

func TestMessagePostedToAnAddressThatIsNotWrittenDown(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post(self.game, "ball_out")
    msg.post(self.game, "bal_out")
end
`, gameHandlingBallOut))

	assertOneProblem(t, problems, unhandledMessageCheck, "main/ball.script", 3,
		`"bal_out" is posted here, but no script in the project handles it`, `did you mean "ball_out"?`)
}

func TestALuaModuleIsJudgedAgainstTheWholeProject(t *testing.T) {
	files := pongWithScripts("", gameHandlingBallOut)
	files["main/referee.lua"] = `local M = {}

function M.whistle(game)
    msg.post(game, "ball_out")
    msg.post("/game#game", "foul")
end

return M
`

	assertOneProblem(t, problemsIn(t, files), unhandledMessageCheck, "main/referee.lua", 5,
		`"foul" is posted here, but no script in the project handles it`)
}

const settingsWithALibrary = `[project]
title = Pong
dependencies#0 = https://github.com/britzl/monarch/archive/master.zip
`

// A library, or a native extension, is code that is not there to be read. It may
// post and handle anything.
func TestAProjectThatRunsCodeOutOfSightIsJudgedOnlyWhereTheReceiverIsKnown(t *testing.T) {
	ballScript := `function update(self, dt)
    msg.post("/game#game", "bal_out")
    msg.post(self.game, "show_popup")
    msg.post("#sprite", "set_skin")
end
`
	outOfSight := map[string]map[string]string{
		"a library":          {"game.project": settingsWithALibrary},
		"a native extension": {"native/ext.manifest": "name: \"native\"\n"},
	}
	for name, extraFiles := range outOfSight {
		t.Run(name, func(t *testing.T) {
			files := pongWithScripts(ballScript, gameHandlingBallOut)
			maps.Copy(files, extraFiles)

			assertOneProblem(t, problemsIn(t, files), unhandledMessageCheck, "main/ball.script", 2, `"bal_out"`)
		})
	}
}

func TestEngineMessagesAreNotTheProjectsToPostOrHandle(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithScripts(`function init(self)
    msg.post(".", "acquire_input_focus")
    msg.post("/game#game", "disable")
end

function on_message(self, message_id, message)
    if message_id == hash("collision_response") then
        self.hit = message.other_id
    end
end
`, "")))
}

func TestHandledMessageThatLooksLikeAnEngineMessage(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function on_message(self, message_id, message)
    if message_id == hash("colision_response") then
        self.hit = message.other_id
    end
end
`, ""))

	assertOneProblem(t, problems, unpostedMessageCheck, "main/ball.script", 2,
		`"colision_response" is handled here, but nothing in the project posts it`, `did you mean "collision_response"?`)
}

// A handler is only known to be dead when every post in the project can be read.
func TestHandledMessagesAreLeftAloneWhenSomePostCannotBeRead(t *testing.T) {
	posts := map[string]string{
		"an id built at run time":      `msg.post("/game#game", "ball_" .. side)`,
		"an id that arrives from afar": `function send(id) msg.post("/game#game", id) end`,
		"msg.post under another name":  `local post = msg.post`,
		"a script that did not parse":  `function init(self) self.speed = end`,
	}
	for name, ballScript := range posts {
		t.Run(name, func(t *testing.T) {
			problems := problemsIn(t, pongWithScripts(ballScript+"\n", gameHandlingBallOut))

			assertNoProblems(t, problemsOf(problems, unpostedMessageCheck))
		})
	}
}

func TestHandledMessagesAreLeftAloneWhenCodeOutOfSightCouldPostThem(t *testing.T) {
	files := pongWithScripts("", gameHandlingBallOut)
	files["game.project"] = settingsWithALibrary

	assertNoProblems(t, problemsIn(t, files))
}

// The engine and the cameras talk to the render script in messages of their own.
func TestTheRenderScriptIsNotAskedWhoPostsToIt(t *testing.T) {
	files := pongWithScripts("", "")
	files["main/pong.render_script"] = `function on_message(self, message_id, message)
    if message_id == hash("set_view_projection") then
        self.view = message.view
    end
end
`

	assertNoProblems(t, problemsIn(t, files))
}

func TestMessageProblemsPointAtTheID(t *testing.T) {
	problems := problemsIn(t, pongWithScripts(`function update(self, dt)
    msg.post("/game#game", "explode")
end
`, ""))

	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %+v", len(problems), problems)
	}
	if column := problems[0].Location.Position.Column; column != strings.Index(`    msg.post("/game#game", "explode")`, `"explode"`)+1 {
		t.Errorf("column = %d, want the column of the message id", column)
	}
}
