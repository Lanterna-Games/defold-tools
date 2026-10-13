package project

import (
	"path"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	prototypeField          = "prototype"
	placedInstancesField    = "instances"
	charactersNoPathHolds   = " \n#"
	charactersOfAPathGlob   = "*?"
	documentThatIsNotNested = ""
)

var fieldsThatHoldFreeText = map[string]bool{"text": true, "value": true, "id": true, "name": true}

func (p *Project) resourceReferencesIn(filePath string, document *textproto.Message) []ResourceReference {
	return p.resourceReferencesUnder(filePath, document, documentThatIsNotNested)
}

func (p *Project) resourceReferencesUnder(filePath string, document *textproto.Message, enclosingField string) []ResourceReference {
	var references []ResourceReference
	for _, field := range document.Fields {
		switch {
		case field.Message != nil:
			references = append(references, p.resourceReferencesUnder(filePath, field.Message, field.Name)...)
		case field.Scalar == nil:
			continue
		case field.Name == embeddedDocumentField && field.Scalar.Quoted:
			embedded := p.parseEmbedded(filePath, field.Scalar)
			references = append(references, p.resourceReferencesUnder(filePath, embedded, documentThatIsNotNested)...)
		case looksLikeResourcePath(field):
			isWhatAFactorySpawns := field.Name == prototypeField && enclosingField != placedInstancesField
			if isWhatAFactorySpawns {
				p.spawnedByAFactory[field.Scalar.Text] = true
			}
			references = append(references, ResourceReference{
				Path:     field.Scalar.Text,
				Location: Location{File: filePath, Position: field.Scalar.Position},
			})
		}
	}
	return references
}

func looksLikeResourcePath(field textproto.Field) bool {
	if !field.Scalar.Quoted || fieldsThatHoldFreeText[field.Name] {
		return false
	}
	value := field.Scalar.Text
	return strings.HasPrefix(value, "/") && path.Ext(value) != "" && !strings.ContainsAny(value, charactersNoPathHolds+charactersOfAPathGlob)
}
