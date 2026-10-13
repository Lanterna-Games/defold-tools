package project

import "github.com/Lanterna-Games/defold-tools/internal/textproto"

const (
	collisionObjectType      = "collisionobject"
	collisionObjectExtension = ".collisionobject"
	tileSourceExtension      = ".tilesource"
)

func (p *Project) collectCollisionObject(filePath string, document *textproto.Message) {
	if group, isSet := document.Scalar("group"); isSet && group.Text != "" {
		p.CollisionGroups[group.Text] = true
		p.declare(DeclaredGroup, "", group.Text, Location{File: filePath, Position: group.Position})
		p.mentionGroup(filePath, group)
	}
	for _, mask := range document.Scalars("mask") {
		if mask.Text == "" {
			continue
		}
		p.mentionGroup(filePath, mask)
		p.CollisionMasks = append(p.CollisionMasks, WrittenID{
			Name:     mask.Text,
			Location: Location{File: filePath, Position: mask.Position},
		})
	}
}

func (p *Project) CollisionGroupNames() []string {
	return sortedKeys(p.CollisionGroups)
}
