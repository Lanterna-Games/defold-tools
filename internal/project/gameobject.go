package project

import (
	"path"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const embeddedDocumentField = "data"

func (p *Project) gameObjectFrom(filePath, resourcePath string, document *textproto.Message) *GameObject {
	gameObject := &GameObject{Path: resourcePath}
	for _, field := range document.Fields {
		if field.Message == nil {
			continue
		}
		switch field.Name {
		case "components":
			resource := field.Message.Text("component")
			gameObject.Components = append(gameObject.Components, Component{
				ID:                field.Message.Text("id"),
				Type:              strings.TrimPrefix(path.Ext(resource), "."),
				Resource:          resource,
				Location:          locationOfID(filePath, field.Message, field.Position),
				IDWhere:           whereTheTextOf(filePath, field.Message, "id"),
				PropertyOverrides: propertyOverridesFrom(filePath, field.Message),
			})
		case "embedded_components":
			component := Component{
				ID:       field.Message.Text("id"),
				Type:     field.Message.Text("type"),
				Location: locationOfID(filePath, field.Message, field.Position),
				IDWhere:  whereTheTextOf(filePath, field.Message, "id"),
			}
			data, hasData := field.Message.Scalar(embeddedDocumentField)
			switch {
			case !hasData:
			case component.Type == collisionObjectType:
				p.collectCollisionObject(filePath, p.parseEmbedded(filePath, data))
			case component.Type == spriteType:
				component.sprite = p.addSprite(spriteFrom(filePath, p.parseEmbedded(filePath, data)))
			}
			gameObject.Components = append(gameObject.Components, component)
		}
	}
	return gameObject
}

func whereTheTextOf(filePath string, message *textproto.Message, field string) Location {
	if scalar, isWritten := message.Scalar(field); isWritten {
		return Location{File: filePath, Position: scalar.TextPosition()}
	}
	return Location{}
}

func locationOfID(filePath string, message *textproto.Message, fallback source.Position) Location {
	if scalar, found := message.Scalar("id"); found {
		return Location{File: filePath, Position: scalar.Position}
	}
	return Location{File: filePath, Position: fallback}
}
