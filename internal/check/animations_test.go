package check

import "testing"

const ballAtlas = `images {
  image: "/main/images/ball.png"
}
images {
  image: "/main/images/ball_glow.png"
}
animations {
  id: "roll"
  images {
    image: "/main/images/ball.png"
  }
  images {
    image: "/main/images/ball_glow.png"
  }
}
`

func ballWithSpriteOf(atlas, defaultAnimation string) string {
	return `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"` + defaultAnimation + `\"\n"
  "material: \"/builtins/materials/sprite.material\"\n"
  "textures {\n"
  "  sampler: \"texture_sampler\"\n"
  "  texture: \"` + atlas + `\"\n"
  "}\n"
  ""
}
`
}

func pongWithAnimations(ballScript string) map[string]string {
	files := pongWithBallScript(ballScript)
	files["main/ball.go"] = ballWithSpriteOf("/main/ball.atlas", "ball")
	files["main/ball.atlas"] = ballAtlas
	files["main/images/ball.png"] = ""
	files["main/images/ball_glow.png"] = ""
	return files
}

func TestAnimationsThatExist(t *testing.T) {
	assertNoProblems(t, problemsIn(t, pongWithAnimations(`local ROLL = hash("roll")

function init(self)
    sprite.play_flipbook("#sprite", "roll")
    sprite.play_flipbook("#sprite", ROLL, function() end)
    sprite.play_flipbook("#sprite", hash("ball_glow"))
    msg.post("#sprite", "play_animation", { id = hash("ball") })
    sprite.play_flipbook(self.other, "roll")
    sprite.play_flipbook("#sprite", "ball_" .. self.state)
end
`+"go.property(\"other\", msg.url())\ngo.property(\"state\", hash(\"glow\"))\n")))
}

func TestAnimationTheAtlasOfTheSpriteDoesNotHave(t *testing.T) {
	problems := problemsIn(t, pongWithAnimations(`function init(self)
    sprite.play_flipbook("#sprite", "rol")
    msg.post("#sprite", "play_animation", { id = hash("bounce") })
end
`))

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownAnimationCheck, "main/ball.script", 2,
		`"rol": "#sprite" plays the animations of /main/ball.atlas, which has no "rol"`, `did you mean "roll"?`)
	assertOneProblem(t, problems[1:], unknownAnimationCheck, "main/ball.script", 3, `/main/ball.atlas, which has no "bounce"`)
}

func TestDefaultAnimationThatIsNotInTheAtlas(t *testing.T) {
	files := pongWithAnimations("")
	files["main/ball.go"] = ballWithSpriteOf("/main/ball.atlas", "bal")
	files["main/paddle.sprite"] = "tile_set: \"/main/ball.atlas\"\ndefault_animation: \"paddle\"\n"

	problems := problemsIn(t, files)

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownAnimationCheck, "main/ball.go", 8,
		`default animation "bal" is not in /main/ball.atlas`, `did you mean "ball"?`)
	assertOneProblem(t, problems[1:], unknownAnimationCheck, "main/paddle.sprite", 2, `default animation "paddle" is not in /main/ball.atlas`)
}

func TestAnimationOfAnAddressThatIsNotWrittenDown(t *testing.T) {
	problems := problemsIn(t, pongWithAnimations(`local function play(target)
    sprite.play_flipbook(target, "roll")
    sprite.play_flipbook(target, "spin")
end
`))

	assertOneProblem(t, problems, unknownAnimationCheck, "main/ball.script", 3,
		`"spin": nothing in the project has an animation "spin"`)
}

func TestTheImagesOfAnAtlasAreRenamedByItsPatterns(t *testing.T) {
	files := pongWithAnimations(`function init(self)
    sprite.play_flipbook("#sprite", "ball")
    sprite.play_flipbook("#sprite", "ball_shine")
    sprite.play_flipbook("#sprite", "ball_glow")
end
`)
	files["main/ball.atlas"] = ballAtlas + "rename_patterns: \"glow=shine,missing=\"\n"

	assertOneProblem(t, problemsIn(t, files), unknownAnimationCheck, "main/ball.script", 4, `"ball_glow"`)
}

func TestTheAnimationsOfATileSource(t *testing.T) {
	files := pongWithAnimations(`function init(self)
    sprite.play_flipbook("#sprite", "walk")
    sprite.play_flipbook("#sprite", "run")
end
`)
	files["main/ball.go"] = ballWithSpriteOf("/main/hero.tilesource", "walk")
	files["main/hero.tilesource"] = "image: \"/main/images/ball.png\"\nanimations {\n  id: \"walk\"\n  start_tile: 1\n  end_tile: 4\n}\n"

	assertOneProblem(t, problemsIn(t, files), unknownAnimationCheck, "main/ball.script", 3, `/main/hero.tilesource, which has no "run"`)
}

func TestAnimationsThatCannotAllBeKnownAreLeftAlone(t *testing.T) {
	script := `function init(self)
    sprite.play_flipbook("#sprite", "anything")
    sprite.play_flipbook(self.other, "anything")
end
` + "go.property(\"other\", msg.url())\n"
	t.Run("the atlas is not in the project", func(t *testing.T) {
		files := pongWithAnimations(script)
		files["main/ball.go"] = ballWithSpriteOf("/library/ball.atlas", "anything")

		assertNoProblems(t, problemsIn(t, files))
	})
	t.Run("an atlas is made while the game runs", func(t *testing.T) {
		assertNoProblems(t, problemsIn(t, pongWithAnimations(script+`
function final(self)
    resource.set_atlas(self.atlas, { animations = {} })
end
go.property("atlas", resource.atlas())
`)))
	})
	t.Run("a library may bring atlases", func(t *testing.T) {
		files := pongWithAnimations(`local function play(target)
    sprite.play_flipbook(target, "anything")
end
`)
		files["game.project"] = settingsWithALibrary

		assertNoProblems(t, problemsIn(t, files))
	})
}

// From Defold 1.14.0 a sprite may have no texture at all, with a material that
// draws without one. There is no atlas to judge an animation against, and
// nothing is missing.
func TestASpriteWithoutATextureIsLeftAlone(t *testing.T) {
	files := pongWithAnimations(`function init(self)
    sprite.play_flipbook("#sprite", "anything")
end
`)
	files["main/ball.go"] = `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "material: \"/builtins/materials/sprite.material\"\n"
  ""
}
`

	assertNoProblems(t, problemsIn(t, files))
}

// A sprite whose image is set from a script plays the animations of whichever
// atlas it was given, so only the animations of the whole project can be ruled out.
func TestASpriteWhoseAtlasIsSwappedIsJudgedAgainstEveryAtlas(t *testing.T) {
	files := pongWithAnimations(`go.property("skin", resource.atlas("/main/skins.atlas"))

function init(self)
    go.set("#sprite", "image", self.skin)
    sprite.play_flipbook("#sprite", "gold")
    sprite.play_flipbook("#sprite", "silver")
end
`)
	files["main/skins.atlas"] = "animations {\n  id: \"gold\"\n}\n"

	assertOneProblem(t, problemsIn(t, files), unknownAnimationCheck, "main/ball.script", 6,
		`"silver": nothing in the project has an animation "silver"`)
}

const hudWithTextures = `script: "/main/hud.gui_script"
textures {
  name: "ui"
  texture: "/main/ball.atlas"
}
nodes {
  id: "icon"
  type: TYPE_BOX
  texture: "ui/ball"
}
nodes {
  id: "badge"
  type: TYPE_BOX
  texture: "ui/bal"
}
nodes {
  id: "other"
  type: TYPE_BOX
  texture: "missing/anything"
}
`

func TestAnimationsOfAGUIScene(t *testing.T) {
	files := pongWithAnimations("")
	files["main/hud.gui"] = hudWithTextures
	files["main/hud.gui_script"] = `function init(self)
    gui.play_flipbook(gui.get_node("icon"), "roll")
    gui.play_flipbook(gui.get_node("icon"), hash("rol"))
end
`

	problems := problemsIn(t, files)

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %+v", len(problems), problems)
	}
	assertOneProblem(t, problems[:1], unknownAnimationCheck, "main/hud.gui", 14,
		`texture "ui/bal": /main/ball.atlas has no animation "bal"`, `did you mean "ball"?`)
	assertOneProblem(t, problems[1:], unknownAnimationCheck, "main/hud.gui_script", 3,
		`"rol": none of the textures of the scene has an animation "rol"`, `did you mean "roll"?`)
}
