package project

import (
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	nodeLookupFunction     = "gui.get_node"
	nodeNamingFunction     = "gui.set_id"
	nodeIDOfALookup        = 0
	nodeIDOfANaming        = 1
	separatorOfATemplateID = "/"
	luaModuleExtension     = ".lua"
)

type GUIScene struct {
	Path         string
	Script       string
	NodeIDs      []string
	Templates    []GUITemplate
	Textures     map[string]string
	NodeTextures []WrittenID
	NodeNames    []WrittenID
	TexturesUsed []WrittenID
}

var fieldsThatNameANode = []string{"id", "parent"}

type GUITemplate struct {
	NodeID string
	Scene  string
}

func (p *Project) guiSceneFrom(filePath, resourcePath string, document *textproto.Message) *GUIScene {
	scene := &GUIScene{Path: resourcePath, Script: document.Text("script"), Textures: map[string]string{}}
	for _, texture := range document.Messages("textures") {
		scene.Textures[texture.Text("name")] = texture.Text("texture")
	}
	for _, node := range document.Messages("nodes") {
		id := node.Text("id")
		if written, isWritten := node.Scalar("id"); isWritten {
			p.declare(DeclaredNode, resourcePath, id, Location{File: filePath, Position: written.Position})
		}
		if texture, hasTexture := node.Scalar("texture"); hasTexture && texture.Text != "" {
			scene.NodeTextures = append(scene.NodeTextures, WrittenID{Name: texture.Text, Location: Location{File: filePath, Position: texture.Position}})
		}
		scene.NodeIDs = append(scene.NodeIDs, id)
		if template := node.Text("template"); template != "" {
			scene.Templates = append(scene.Templates, GUITemplate{NodeID: id, Scene: template})
		}
	}
	nodesOfLayouts := []*textproto.Message{}
	for _, layout := range document.Messages("layouts") {
		nodesOfLayouts = append(nodesOfLayouts, layout.Messages("nodes")...)
	}
	for _, node := range append(document.Messages("nodes"), nodesOfLayouts...) {
		for _, field := range fieldsThatNameANode {
			if written, isWritten := node.Scalar(field); isWritten && written.Text != "" {
				scene.NodeNames = append(scene.NodeNames, WrittenID{Name: written.Text, Location: Location{File: filePath, Position: written.TextPosition()}})
			}
		}
		if texture, hasTexture := node.Scalar("texture"); hasTexture && texture.Text != "" {
			scene.TexturesUsed = append(scene.TexturesUsed, WrittenID{Name: texture.Text, Location: Location{File: filePath, Position: texture.TextPosition()}})
		}
	}
	return scene
}

type GUINodes struct {
	ids                        map[string]bool
	prefixesThatCouldNotBeRead []string
}

func (n GUINodes) Has(id string) bool {
	return n.ids[id]
}

func (n GUINodes) CouldHoldMoreThanWasRead(id string) bool {
	for _, prefix := range n.prefixesThatCouldNotBeRead {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func (n GUINodes) IDs() []string {
	return sortedKeys(n.ids)
}

func (p *Project) NodesOf(scene *GUIScene) GUINodes {
	nodes := GUINodes{ids: map[string]bool{}}
	p.addNodes(&nodes, scene, "", map[*GUIScene]bool{})
	return nodes
}

func (p *Project) addNodes(nodes *GUINodes, scene *GUIScene, prefix string, ancestors map[*GUIScene]bool) {
	if ancestors[scene] {
		return
	}
	ancestors[scene] = true
	defer delete(ancestors, scene)

	for _, id := range scene.NodeIDs {
		nodes.ids[prefix+id] = true
	}
	for _, template := range scene.Templates {
		templatePrefix := prefix + template.NodeID + separatorOfATemplateID
		if templateScene, isLoaded := p.GUIScenes[template.Scene]; isLoaded {
			p.addNodes(nodes, templateScene, templatePrefix, ancestors)
		} else {
			nodes.prefixesThatCouldNotBeRead = append(nodes.prefixesThatCouldNotBeRead, templatePrefix)
		}
	}
}

func (s *Script) collectGUINodes() {
	lua.Inspect(s.Chunk, func(node lua.Node) bool {
		call, isCall := node.(*lua.CallExpression)
		if !isCall {
			return true
		}
		if arguments, looksANodeUp := argumentsOf(call, nodeLookupFunction); looksANodeUp && len(arguments) > nodeIDOfALookup {
			if id, isWritten := s.IDWrittenIn(arguments[nodeIDOfALookup]); isWritten {
				s.GUINodesLookedUp = append(s.GUINodesLookedUp, WrittenID{Name: id, Location: s.LocationOf(arguments[nodeIDOfALookup])})
			} else {
				s.cannotRead(arguments[nodeIDOfALookup], "the id of the node asked for here cannot be read")
			}
		}
		if arguments, namesANode := argumentsOf(call, nodeNamingFunction); namesANode && len(arguments) > nodeIDOfANaming {
			if id, isWritten := s.IDWrittenIn(arguments[nodeIDOfANaming]); isWritten {
				s.GUINodesItNames = append(s.GUINodesItNames, id)
			} else {
				s.NamesGUINodesThatCannotBeRead = true
				s.cannotRead(arguments[nodeIDOfANaming], "the id given to this node cannot be read, so the nodes this script asks for are not judged")
			}
		}
		return true
	})
}

func (s *Script) IsALuaModule() bool {
	return strings.HasSuffix(s.Path, luaModuleExtension)
}
