package project

import (
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	atlasExtension              = ".atlas"
	spriteExtension             = ".sprite"
	particleEffectExtension     = ".particlefx"
	spriteType                  = "sprite"
	separatorOfATextureAndImage = "/"
)

func (p *Project) animationsOfAnAtlas(filePath, resourcePath string, document *textproto.Message) []string {
	var names []string
	for _, image := range document.Messages("images") {
		name := atlas.NameOfAnImage(image.Text("image"), document.Text("rename_patterns"))
		names = append(names, name)
		if written, isWritten := image.Scalar("image"); isWritten {
			p.declare(DeclaredAnimation, resourcePath, name, Location{File: filePath, Position: written.Position})
		}
	}
	return append(names, p.animationIDsIn(filePath, resourcePath, document)...)
}

func (p *Project) animationIDsIn(filePath, resourcePath string, document *textproto.Message) []string {
	var ids []string
	for _, animation := range document.Messages("animations") {
		ids = append(ids, animation.Text("id"))
		if written, isWritten := animation.Scalar("id"); isWritten {
			p.declare(DeclaredAnimation, resourcePath, written.Text, Location{File: filePath, Position: written.Position})
			p.mention(DeclaredAnimation, resourcePath, written.Text, Location{File: filePath, Position: written.TextPosition()})
		}
	}
	return ids
}

type Sprite struct {
	Textures              []string
	DefaultAnimation      WrittenID
	DefaultAnimationWhere Location
}

type EmitterAnimation struct {
	TileSource string
	Name       string
	Where      Location
}

func (p *Project) collectEmitters(filePath string, document *textproto.Message) {
	for _, emitter := range document.Messages("emitters") {
		if animation, isSet := emitter.Scalar("animation"); isSet && animation.Text != "" {
			p.EmitterAnimations = append(p.EmitterAnimations, EmitterAnimation{
				TileSource: emitter.Text("tile_source"),
				Name:       animation.Text,
				Where:      Location{File: filePath, Position: animation.TextPosition()},
			})
		}
	}
}

func spriteFrom(filePath string, document *textproto.Message) *Sprite {
	sprite := &Sprite{}
	if tileSet := document.Text("tile_set"); tileSet != "" {
		sprite.Textures = append(sprite.Textures, tileSet)
	}
	for _, texture := range document.Messages("textures") {
		if resource := texture.Text("texture"); resource != "" {
			sprite.Textures = append(sprite.Textures, resource)
		}
	}
	if animation, isSet := document.Scalar("default_animation"); isSet {
		sprite.DefaultAnimation = WrittenID{Name: animation.Text, Location: Location{File: filePath, Position: animation.Position}}
		sprite.DefaultAnimationWhere = Location{File: filePath, Position: animation.TextPosition()}
	}
	return sprite
}

func (p *Project) addSprite(sprite *Sprite) *Sprite {
	p.Sprites = append(p.Sprites, sprite)
	return sprite
}

func (p *Project) SpriteOf(component Component) (sprite *Sprite, isASpriteThatWasRead bool) {
	if component.Type != spriteType {
		return nil, false
	}
	if component.sprite != nil {
		return component.sprite, true
	}
	sprite, isRead := p.spriteFiles[component.Resource]
	return sprite, isRead
}

func (p *Project) AnimationsOf(textures []string) (names []string, areAllRead bool) {
	if len(textures) == 1 {
		names, areAllRead = p.Animations[textures[0]]
		return names, areAllRead
	}
	for _, texture := range textures {
		animations, isRead := p.Animations[texture]
		if !isRead {
			return nil, false
		}
		names = append(names, animations...)
	}
	return names, len(textures) > 0
}

func (p *Project) EveryAnimation() []string {
	var names []string
	for _, resource := range sortedKeys(p.Animations) {
		names = append(names, p.Animations[resource]...)
	}
	return names
}

func TextureAndAnimation(nodeTexture string) (texture, animation string, namesAnAnimation bool) {
	return strings.Cut(nodeTexture, separatorOfATextureAndImage)
}
