package check

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	unknownAnimationCheck   = "unknown-animation"
	spritePlayFunction      = "sprite.play_flipbook"
	guiPlayFunction         = "gui.play_flipbook"
	postFunction            = "msg.post"
	playAnimationMessage    = "play_animation"
	animationOfAPlayMessage = "id"
	propertyOfTheTexture    = "image"
)

var functionsThatMakeAnAtlasAtRunTime = map[string]bool{"resource.create_atlas": true, "resource.set_atlas": true}

var texturesWithoutAnimations = map[string]bool{".png": true, ".jpg": true, ".jpeg": true}

type animationUse struct {
	name            string
	location        project.Location
	target          string
	targetIsWritten bool
}

func animationUsesIn(script *project.Script) (ofSprites, ofGUINodes []animationUse) {
	use := func(animation, target lua.Expression) (animationUse, bool) {
		name, isWritten := script.IDWrittenIn(animation)
		if !isWritten {
			return animationUse{}, false
		}
		played := animationUse{name: name, location: script.LocationOf(animation)}
		if target != nil {
			if address, targetIsWritten := script.AddressWrittenIn(target); targetIsWritten {
				played.target, played.targetIsWritten = address.Text, true
			}
		}
		return played, true
	}
	lua.Inspect(script.Chunk, func(node lua.Node) bool {
		call, isCall := node.(*lua.CallExpression)
		if !isCall || call.Method != nil || len(call.Arguments) < 2 {
			return true
		}
		function, _ := lua.QualifiedName(call.Function)
		switch function {
		case spritePlayFunction:
			if played, isWritten := use(call.Arguments[1], call.Arguments[0]); isWritten {
				ofSprites = append(ofSprites, played)
			}
		case guiPlayFunction:
			if played, isWritten := use(call.Arguments[1], nil); isWritten {
				ofGUINodes = append(ofGUINodes, played)
			}
		case postFunction:
			if animation, isAPlayMessage := animationOfThePlayMessage(script, call); isAPlayMessage {
				if played, isWritten := use(animation, call.Arguments[0]); isWritten {
					ofSprites = append(ofSprites, played)
				}
			}
		}
		return true
	})
	return ofSprites, ofGUINodes
}

func animationOfThePlayMessage(script *project.Script, post *lua.CallExpression) (animation lua.Expression, isAPlayMessage bool) {
	if len(post.Arguments) < 3 {
		return nil, false
	}
	if message, isWritten := script.IDWrittenIn(post.Arguments[1]); !isWritten || message != playAnimationMessage {
		return nil, false
	}
	fields, isATable := post.Arguments[2].(*lua.TableLiteral)
	if !isATable {
		return nil, false
	}
	for _, field := range fields.Fields {
		if key, isNamed := field.Key.(*lua.StringLiteral); isNamed && key.Value == animationOfAPlayMessage {
			return field.Value, true
		}
	}
	return nil, false
}

type animationJudge struct {
	inspection
	verdicts                verdicts
	everyAnimation          []string
	everyAnimationIsInSight bool
	spritesKeepTheirTexture bool
}

func texturesThatChangeAtRunTime(inspected inspection) (atlasesAreMade, imagesAreSet bool) {
	for _, script := range inspected.project.Scripts {
		lua.Inspect(script.Chunk, func(node lua.Node) bool {
			call, isCall := node.(*lua.CallExpression)
			if !isCall {
				return true
			}
			if function, _ := lua.QualifiedName(call.Function); functionsThatMakeAnAtlasAtRunTime[function] {
				atlasesAreMade = true
			}
			return true
		})
		for _, use := range propertyUsesIn(script, inspected.reference) {
			if use.name == propertyOfTheTexture {
				imagesAreSet = true
			}
		}
	}
	return atlasesAreMade, imagesAreSet
}

func unknownAnimations(inspected inspection) []Problem {
	loaded := inspected.project
	judge := animationJudge{
		inspection:              inspected,
		verdicts:                newVerdicts(unknownAnimationCheck),
		everyAnimation:          loaded.EveryAnimation(),
		everyAnimationIsInSight: !loaded.RunsCodeOutOfSight,
	}
	for _, sprite := range loaded.Sprites {
		if _, areAllRead := loaded.AnimationsOf(sprite.Textures); !areAllRead && len(sprite.Textures) > 0 {
			judge.everyAnimationIsInSight = false
		}
	}
	atlasesAreMade, imagesAreSet := texturesThatChangeAtRunTime(inspected)
	judge.everyAnimationIsInSight = judge.everyAnimationIsInSight && !atlasesAreMade
	judge.spritesKeepTheirTexture = !imagesAreSet
	problems := slices.Concat(judge.defaultAnimationsThatAreMissing(), judge.nodeTexturesThatAreMissing())

	hasAnOwner, ofSpritesIn := map[string]bool{}, map[string][]animationUse{}
	for _, owner := range loaded.Owners() {
		for _, scriptPath := range loaded.ScriptPathsOf(owner.GameObject) {
			script, isLoaded := loaded.Scripts[scriptPath]
			if !isLoaded {
				continue
			}
			if !hasAnOwner[scriptPath] {
				ofSpritesIn[scriptPath], _ = animationUsesIn(script)
			}
			hasAnOwner[scriptPath] = true
			for _, use := range ofSpritesIn[scriptPath] {
				judge.judge(use, &owner)
			}
		}
	}
	for _, scriptPath := range loaded.ScriptPaths() {
		script := loaded.Scripts[scriptPath]
		ofSprites, ofGUINodes := animationUsesIn(script)
		if !hasAnOwner[scriptPath] {
			for _, use := range ofSprites {
				judge.judge(use, nil)
			}
		}
		for _, use := range ofGUINodes {
			judge.judgeOfAGUINode(use, script)
		}
	}
	return append(problems, judge.verdicts.problems()...)
}

func describedTextures(textures []string) string {
	if len(textures) == 1 {
		return textures[0]
	}
	return "any of " + quotedList(textures)
}

func (j animationJudge) defaultAnimationsThatAreMissing() []Problem {
	var problems []Problem
	for _, sprite := range j.project.Sprites {
		animation := sprite.DefaultAnimation
		names, areAllRead := j.project.AnimationsOf(sprite.Textures)
		if animation.Name == "" || !areAllRead || slices.Contains(names, animation.Name) {
			continue
		}
		problems = append(problems, Problem{
			Check:    unknownAnimationCheck,
			Location: animation.Location,
			Message: fmt.Sprintf("default animation %q is not in %s%s",
				animation.Name, describedTextures(sprite.Textures), hintTowards(animation.Name, names, "%q")),
		})
	}
	return problems
}

func (j animationJudge) nodeTexturesThatAreMissing() []Problem {
	var problems []Problem
	for _, scenePath := range sortedScenePaths(j.project) {
		scene := j.project.GUIScenes[scenePath]
		for _, nodeTexture := range scene.NodeTextures {
			texture, animation, namesAnAnimation := project.TextureAndAnimation(nodeTexture.Name)
			names, isRead := j.project.Animations[scene.Textures[texture]]
			if !namesAnAnimation || !isRead || slices.Contains(names, animation) {
				continue
			}
			problems = append(problems, Problem{
				Check:    unknownAnimationCheck,
				Location: nodeTexture.Location,
				Message: fmt.Sprintf("texture %q: %s has no animation %q%s",
					nodeTexture.Name, scene.Textures[texture], animation, hintTowards(animation, names, "%q")),
			})
		}
	}
	return problems
}

func (j animationJudge) judge(use animationUse, owner *project.Owner) {
	if use.targetIsWritten && owner != nil && j.spritesKeepTheirTexture && j.everyAnimationIsInSight {
		if sprite, isInSight := j.spriteAt(project.ParseURL(use.target), *owner); isInSight {
			names, areAllRead := j.project.AnimationsOf(sprite.Textures)
			if !areAllRead || slices.Contains(names, use.name) {
				j.verdicts.accept(use.location)
				return
			}
			j.verdicts.reject(use.location, fmt.Sprintf("%q: %q plays the animations of %s, which has no %q%s",
				use.name, use.target, describedTextures(sprite.Textures), use.name, hintTowards(use.name, names, "%q")))
			return
		}
	}
	j.judgeAgainstEveryAnimation(use, j.everyAnimation, j.everyAnimationIsInSight)
}

func (j animationJudge) judgeAgainstEveryAnimation(use animationUse, names []string, areAllInSight bool) {
	if !areAllInSight || len(names) == 0 || slices.Contains(names, use.name) {
		j.verdicts.accept(use.location)
		return
	}
	j.verdicts.reject(use.location, fmt.Sprintf("%q: nothing in the project has an animation %q%s",
		use.name, use.name, hintTowards(use.name, names, "%q")))
}

func (j animationJudge) spriteAt(url project.URL, owner project.Owner) (sprite *project.Sprite, isInSight bool) {
	if !url.HasFragment || url.Fragment == "" {
		return nil, false
	}
	target, isKnown := j.project.TargetOf(owner, url)
	if !isKnown {
		return nil, false
	}
	component, exists := target.GameObject.Component(url.Fragment)
	if !exists {
		return nil, false
	}
	return j.project.SpriteOf(component)
}

func (j animationJudge) judgeOfAGUINode(use animationUse, script *project.Script) {
	var scenes []*project.GUIScene
	for _, scenePath := range sortedScenePaths(j.project) {
		if scene := j.project.GUIScenes[scenePath]; scene.Script == script.Path {
			scenes = append(scenes, scene)
		}
	}
	areAllInSight := true
	if len(scenes) == 0 {
		areAllInSight = !j.project.RunsCodeOutOfSight
		for _, scenePath := range sortedScenePaths(j.project) {
			scenes = append(scenes, j.project.GUIScenes[scenePath])
		}
	}

	var names, textures []string
	for _, scene := range scenes {
		for _, resource := range scene.Textures {
			animations, isRead := j.project.Animations[resource]
			if !isRead && !texturesWithoutAnimations[strings.ToLower(path.Ext(resource))] {
				areAllInSight = false
			}
			names = append(names, animations...)
			textures = append(textures, resource)
		}
		if len(scene.Templates) > 0 && j.project.RunsCodeOutOfSight {
			areAllInSight = false
		}
	}
	if !areAllInSight || len(textures) == 0 || slices.Contains(names, use.name) {
		j.verdicts.accept(use.location)
		return
	}
	slices.Sort(textures)
	j.verdicts.reject(use.location, fmt.Sprintf("%q: none of the textures of the scene has an animation %q%s",
		use.name, use.name, hintTowards(use.name, names, "%q")))
}
