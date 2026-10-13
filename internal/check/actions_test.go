package check

import (
	"maps"
	"testing"
)

const bindingOfJumpAndTouch = `key_trigger {
  input: KEY_SPACE
  action: "jump"
}
mouse_trigger {
  input: MOUSE_BUTTON_1
  action: "touch"
}
`

func pongWithInput(ballScript string) map[string]string {
	files := pongWithBallScript(ballScript)
	files["input/game.input_binding"] = bindingOfJumpAndTouch
	return files
}

func TestActionsThatAreBound(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithInput(`local TOUCH = hash("touch")

function on_input(self, action_id, action)
    if action_id == hash("jump") and action.pressed then
        self.jumping = true
    elseif action_id == TOUCH then
        self.touched = true
    elseif action_id == nil then
        self.moved = true
    elseif action_id ~= action.previous then
        self.changed = true
    end
end
`)))
}

func TestActionThatIsNotBound(t *testing.T) {
	problems := problemsIn(t, pongWithInput(`local FIRE = hash("fire")

function on_input(self, action_id, action)
    if action_id == hash("jmup") then
        self.jumping = true
    elseif FIRE == action_id then
        self.firing = true
    end
    if action_id ~= hash("tuoch") then
        return
    end
end
`))

	if len(problems) != 3 {
		t.Fatalf("got %d problems, want 3: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unboundActionCheck, "main/ball.script", 4,
		`"jmup" is not bound in /input/game.input_binding`, `did you mean "jump"?`)
	assertOneProblem(t, problems[1:2], unboundActionCheck, "main/ball.script", 6, `"fire" is not bound in /input/game.input_binding`)
	assertOneProblem(t, problems[2:], unboundActionCheck, "main/ball.script", 9, `did you mean "touch"?`)
}

func TestActionInAGUIScript(t *testing.T) {
	files := pongWithInput("")
	files["main/hud.gui_script"] = `function on_input(self, action_id, action)
    if action_id == hash("click") then
        self.clicked = true
    end
end
`

	assertOneProblem(t, problemsIn(t, files), unboundActionCheck, "main/hud.gui_script", 2, `"click"`)
}

func TestTheGameBindingIsTheOneTheSettingsName(t *testing.T) {
	files := pongWithBallScript(`function on_input(self, action_id, action)
    if action_id == hash("jump") or action_id == hash("crouch") then
        self.moving = true
    end
end
`)
	files["game.project"] = "[input]\ngame_binding = /controls/pong.input_bindingc\n"
	files["controls/pong.input_binding"] = bindingOfJumpAndTouch

	assertOneProblem(t, problemsIn(t, files), unboundActionCheck, "main/ball.script", 2,
		`"crouch" is not bound in /controls/pong.input_binding`)
}

func TestTheBindingThatComesWithTheEngine(t *testing.T) {
	files := pongWithBallScript(`function on_input(self, action_id, action)
    if action_id == hash("key_space") or action_id == hash("mouse_button_left") then
        self.pressed = true
    elseif action_id == hash("key_spcae") then
        self.pressed = false
    end
end
`)
	files["game.project"] = "[input]\ngame_binding = /builtins/input/all.input_bindingc\n"

	assertOneProblem(t, problemsIn(t, files), unboundActionCheck, "main/ball.script", 4,
		`"key_spcae" is not bound in /builtins/input/all.input_binding`, `did you mean "key_space"?`)
}

// With several bindings in the project, any of them may be the one in use for
// some build, so an action bound in one of them is taken to be bound.
func TestAnActionBoundInAnyBindingOfTheProjectIsBound(t *testing.T) {
	files := pongWithInput(`function on_input(self, action_id, action)
    if action_id == hash("debug_menu") or action_id == hash("nothing") then
        self.debugging = true
    end
end
`)
	files["input/debug.input_binding"] = `key_trigger {
  input: KEY_F1
  action: "debug_menu"
}
`

	assertOneProblem(t, problemsIn(t, files), unboundActionCheck, "main/ball.script", 2,
		`"nothing" is not bound in any input binding of the project`)
}

func TestActionsAreLeftAloneWhenTheGameBindingCannotBeRead(t *testing.T) {
	script := `function on_input(self, action_id, action)
    if action_id == hash("anything") then
        self.pressed = true
    end
end
`
	bindings := map[string]map[string]string{
		"no binding at all":        {},
		"a binding from a library": {"game.project": "[input]\ngame_binding = /library/input/game.input_bindingc\n", "input/game.input_binding": bindingOfJumpAndTouch},
	}
	for name, extraFiles := range bindings {
		t.Run(name, func(t *testing.T) {
			files := pongWithBallScript(script)
			maps.Copy(files, extraFiles)

			assertNoProblems(t, problemsIn(t, files))
		})
	}
}

// A script that calls its own on_input makes up input the bindings know nothing
// about.
func TestAnActionTheScriptFeedsItselfIsNotExpectedToBeBound(t *testing.T) {
	problems := problemsIn(t, pongWithInput(`function on_input(self, action_id, action)
    if action_id == hash("virtual_jump") or action_id == hash("virtual_fire") then
        self.acting = true
    end
end

function on_message(self, message_id, message)
    if message_id == hash("button_pressed") then
        on_input(self, hash("virtual_jump"), { pressed = true })
    end
end
`))

	assertOneProblem(t, problemsOf(problems, unboundActionCheck), unboundActionCheck, "main/ball.script", 2, `"virtual_fire"`)
}

func TestAScriptThatFeedsItselfActionsThatCannotBeReadIsNotJudged(t *testing.T) {
	problems := problemsIn(t, pongWithInput(`function on_input(self, action_id, action)
    if action_id == hash("virtual_jump") then
        self.acting = true
    end
end

function on_message(self, message_id, message)
    on_input(self, message.action_id, message.action)
end
`))

	assertNoProblems(t, problemsOf(problems, unboundActionCheck))
}

func TestOnlyTheInputHandlerOfTheScriptIsRead(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithInput(`local M = {}

function M.on_input(self, action_id, action)
    return action_id == hash("from_a_module")
end

local function handle(self, action_id, action)
    return action_id == hash("from_a_helper")
end

function on_message(self, message_id, message)
    local action_id = message.action_id
    return action_id == hash("not_input")
end
`)))
}
