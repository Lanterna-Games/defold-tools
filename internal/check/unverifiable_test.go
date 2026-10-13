package check

import (
	"strings"
	"testing"
)

func strictProblemsIn(t *testing.T, files map[string]string) []Problem {
	t.Helper()
	return problemsOf(problemsWith(t, files, Options{}), unverifiableCheck)
}

func TestWhatIsWrittenDownIsVerifiable(t *testing.T) {
	files := pongWithHUD(`local SCORE = hash("score")

function init(self)
    self.score = gui.get_node(SCORE)
    gui.play_flipbook(self.score, "roll")
end

function on_input(self, action_id, action)
    if action_id == hash("touch") or action_id == nil then
        return true
    end
end
`)
	files["main/ball.script"] = `local ids = require("main.ids")

function update(self, dt)
    msg.post("/game#game", ids.BALL_OUT, { side = 1 })
    msg.post(self.game, "serve")
    go.set(".", "position.x", 0)
    go.set(self.game, ids.SCORE, 1)
    sprite.play_flipbook("#sprite", hash("roll"))
    physics.raycast(go.get_position(), vmath.vector3(), { hash("wall") })
    go.delete(self.spawned)
    msg.post(msg.url(nil, self.spawned, "script"), "serve")
end

function on_message(self, message_id, message, sender)
    if message_id == ids.BALL_OUT then
        msg.post(sender, "serve")
    elseif message_id ~= hash("serve") then
        return
    end
end
` + ballScriptDeclaringGame
	files["main/ids.lua"] = "local M = {}\nM.BALL_OUT = hash(\"ball_out\")\nM.SCORE = \"score\"\nreturn M\n"

	assertNoProblems(t, strictProblemsIn(t, files))
}

func TestNamesThatAreNotWrittenDownAreUnverifiable(t *testing.T) {
	uses := map[string]struct {
		line string
		want string
	}{
		"a message id":              {line: `msg.post("/game", self.next_message)`, want: "the id of the message posted here cannot be read"},
		"a message id put together": {line: `msg.post("/game", "ball_" .. self.side)`, want: "the id of the message posted here cannot be read"},
		"msg.post as a value":       {line: `local post = msg.post`, want: "msg.post is used here as a value"},
		"a property":                {line: `go.set(".", self.property, 1)`, want: "the name of the property cannot be read"},
		"an animation":              {line: `sprite.play_flipbook("#sprite", self.animation)`, want: "the animation played here cannot be read"},
		"an animation in a message": {line: `msg.post("#sprite", "play_animation", { id = self.animation })`, want: "the animation played here cannot be read"},
		"a collision group":         {line: `physics.set_maskbit("#co", self.group, true)`, want: "the collision group named here cannot be read"},
		"the groups of a ray cast":  {line: `physics.raycast(go.get_position(), vmath.vector3(), self.groups)`, want: "the collision group named here cannot be read"},
		"one group of a ray cast":   {line: `physics.raycast(go.get_position(), vmath.vector3(), { hash("wall"), self.group })`, want: "the collision group named here cannot be read"},
		"an address put together":   {line: `msg.post("/enemy" .. self.index, "disable")`, want: "this address is put together while the game runs"},
		"an address formatted":      {line: `go.delete(string.format("/enemy%d", self.index))`, want: "this address is put together while the game runs"},
		"a node":                    {line: `gui.get_node("item_" .. self.index)`, want: "the id of the node asked for here cannot be read"},
		"a node that is named":      {line: `gui.set_id(self.node, "item_" .. self.index)`, want: "the id given to this node cannot be read, so the nodes this script asks for are not judged"},
	}
	for name, testCase := range uses {
		t.Run(name, func(t *testing.T) {
			problems := strictProblemsIn(t, pongWithBallScript("function update(self, dt)\n    "+testCase.line+"\nend\n"))

			assertOneProblem(t, problems, unverifiableCheck, "main/ball.script", 2, testCase.want)
		})
	}
}

func TestAHandlerThatCouldHandleAnyMessageIsUnverifiableWhereItSaysSo(t *testing.T) {
	handlers := map[string]struct {
		body     string
		wantLine int
		want     string
	}{
		"passes the id on":           {body: "    dispatch(message_id, message)", wantLine: 2, want: "the message id is used here for something other than a comparison"},
		"compares with an unread id": {body: "    if message_id == self.expected then self.ok = true end", wantLine: 2, want: "the id compared here cannot be read"},
		"has an else":                {body: "    if message_id == hash(\"a\") then\n        self.a = true\n    else\n        self.other = true\n    end", wantLine: 5, want: "every other message falls into this else"},
		"all but one":                {body: "    if message_id ~= hash(\"a\") then self.other = true end", wantLine: 2, want: "this branch runs for every message but one"},
		"never looks":                {body: "    go.delete()", wantLine: 2, want: "on_message acts without comparing the message id with one that can be read"},
	}
	for name, testCase := range handlers {
		t.Run(name, func(t *testing.T) {
			problems := strictProblemsIn(t, pongWithBallScript("function on_message(self, message_id, message, sender)\n"+testCase.body+"\nend\n"))

			assertOneProblem(t, problems, unverifiableCheck, "main/ball.script", testCase.wantLine, testCase.want, "so this script counts as handling every message")
		})
	}
}

func TestOnlyTheFirstReasonAHandlerHandlesEverythingIsReported(t *testing.T) {
	problems := strictProblemsIn(t, pongWithBallScript(`function on_message(self, message_id, message, sender)
    log(message_id)
    dispatch(message_id, message)
    forward(message_id)
end
`))

	assertOneProblem(t, problems, unverifiableCheck, "main/ball.script", 2, "the message id is used here")
}

func TestInputThatCannotBeReadIsUnverifiable(t *testing.T) {
	problems := strictProblemsIn(t, pongWithBallScript(`function on_input(self, action_id, action)
    if action_id == self.jump_action then
        on_input(self, self.other_action, action)
    end
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	if !strings.Contains(problems[0].Message, "the action compared here cannot be read") || !strings.Contains(problems[1].Message, "the action passed to on_input here cannot be read") {
		t.Errorf("problems = %+v", problems)
	}
}

func TestStrictModeIsACheckLikeAnyOther(t *testing.T) {
	files := pongWithBallScript(`function update(self, dt)
    go.set(".", self.property, 1) -- defold-vet:ignore unverifiable the property comes from the level file
    go.set(".", self.other, 1)
end
`)

	assertOneProblem(t, problemsOf(problemsWith(t, files, Options{}), unverifiableCheck), unverifiableCheck, "main/ball.script", 3, "the name of the property cannot be read")
	assertNoProblems(t, problemsOf(problemsWith(t, files, Options{TurnedOff: map[string]bool{unverifiableCheck: true}}), unverifiableCheck))
}
