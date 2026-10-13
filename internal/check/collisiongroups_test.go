package check

import "testing"

const wallThatStopsTheBall = `embedded_components {
  id: "collisionobject"
  type: "collisionobject"
  data: "type: COLLISION_OBJECT_TYPE_STATIC\n"
  "group: \"wall\"\n"
  "mask: \"ball\"\n"
  ""
}
`

const ballThatHitsWallsAndPaddles = `embedded_components {
  id: "collisionobject"
  type: "collisionobject"
  data: "type: COLLISION_OBJECT_TYPE_KINEMATIC\n"
  "group: \"ball\"\n"
  "mask: \"wall\"\n"
  "mask: \"paddle\"\n"
  ""
}
`

func pongWithCollisions(ballScript string) map[string]string {
	files := pongWithBallScript(ballScript)
	files["main/ball.go"] = ballWithSpriteAndScript + ballThatHitsWallsAndPaddles
	files["main/wall.go"] = wallThatStopsTheBall
	files["main/paddle.collisionobject"] = "group: \"paddle\"\nmask: \"ball\"\n"
	return files
}

func TestCollisionGroupsThatExist(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithCollisions(`local WALL = hash("wall")
local SOLID = { WALL, hash("paddle") }

function update(self, dt)
    self.hit = physics.raycast(go.get_position(), vmath.vector3(), { hash("wall"), hash("paddle") })
    physics.raycast_async(go.get_position(), vmath.vector3(), SOLID, 1)
    physics.set_group("#collisionobject", "ball")
    physics.set_maskbit("#collisionobject", "paddle", false)
end

function on_message(self, message_id, message, sender)
    if message_id == hash("collision_response") and message.other_group == WALL then
        self.bounced = true
    elseif message.own_group == hash("ball") or message.group == hash("paddle") then
        self.scored = true
    end
end
`)))
}

func TestCollisionGroupNoCollisionObjectHas(t *testing.T) {
	problems := problemsIn(t, pongWithCollisions(`function update(self, dt)
    self.hit = physics.raycast(go.get_position(), vmath.vector3(), { hash("wall"), hash("padle") })
    physics.set_maskbit("#collisionobject", "floor", false)
end

function on_message(self, message_id, message, sender)
    if message.other_group == hash("wal") then
        self.bounced = true
    elseif hash("enemy") == message.group then
        self.hurt = true
    end
end
`))

	if len(problems) != 4 {
		t.Fatalf("got %d problems, want 4: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownCollisionGroupCheck, "main/ball.script", 2,
		`group "padle" is not the group of any collision object`, `did you mean "paddle"?`)
	assertOneProblem(t, problems[1:2], unknownCollisionGroupCheck, "main/ball.script", 3, `group "floor"`)
	assertOneProblem(t, problems[2:3], unknownCollisionGroupCheck, "main/ball.script", 7, `group "wal"`, `did you mean "wall"?`)
	assertOneProblem(t, problems[3:], unknownCollisionGroupCheck, "main/ball.script", 9, `group "enemy"`)
}

func TestMaskThatNamesAGroupNoCollisionObjectHas(t *testing.T) {
	files := pongWithCollisions("")
	files["main/wall.go"] = `embedded_components {
  id: "collisionobject"
  type: "collisionobject"
  data: "type: COLLISION_OBJECT_TYPE_STATIC\n"
  "group: \"wall\"\n"
  "mask: \"bal\"\n"
  "mask: \"\"\n"
  ""
}
`

	assertOneProblem(t, problemsIn(t, files), unknownCollisionGroupCheck, "main/wall.go", 6,
		`mask "bal" is not the group of any collision object`, `did you mean "ball"?`)
}

func TestTheGroupsOfATileSourceAreGroups(t *testing.T) {
	files := pongWithCollisions(`function on_message(self, message_id, message, sender)
    if message.other_group == hash("ground") then
        self.landed = true
    end
end
`)
	files["main/level.tilesource"] = "image: \"/main/tiles.png\"\ncollision_groups: \"ground\"\ncollision_groups: \"spikes\"\n"
	files["main/tiles.png"] = ""

	assertNoProblems(t, problemsIn(t, files))
}

// A field called group is a collision group only in the message the engine sends.
func TestAGroupThatIsNotACollisionGroupIsLeftAlone(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithCollisions(`local function same_team(a, b)
    return a.group == hash("red") and b.group ~= "blue"
end

function update(self, dt)
    sound.set_group_gain("music", 0.5)
    self.built = physics.raycast(go.get_position(), vmath.vector3(), self.groups)
    self.dynamic = physics.raycast(go.get_position(), vmath.vector3(), { hash("gr" .. "ound") })
end

function on_message(self, message_id, message, sender)
    if message.team == hash("red") or sender.group == hash("blue") then
        self.friendly = true
    end
end
`+"go.property(\"groups\", 1)\n")))
}

func TestCollisionGroupsAreLeftAloneWhenTheyCannotAllBeKnown(t *testing.T) {
	script := `function on_message(self, message_id, message, sender)
    if message.other_group == hash("enemy") then
        self.hurt = true
    end
end
`
	t.Run("nothing in the project declares a group", func(t *testing.T) {
		assertNoProblems(t, problemsIn(t, pongWithBallScript(script)))
	})
	t.Run("a library may declare more", func(t *testing.T) {
		files := pongWithCollisions(script)
		files["game.project"] = settingsWithALibrary

		assertNoProblems(t, problemsIn(t, files))
	})
}
