package scene

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	ExtensionOfAGUI = ".gui"

	KindOfAPanel     = "panel"
	KindOfAPie       = "pie"
	KindOfTheDisplay = "display"

	typeOfABoxNode      = "TYPE_BOX"
	typeOfATextNode     = "TYPE_TEXT"
	typeOfAPieNode      = "TYPE_PIE"
	typeOfATemplateNode = "TYPE_TEMPLATE"
	sizeTakenFromImage  = "SIZE_MODE_AUTO"
	boundsOfASquarePie  = "PIEBOUNDS_RECTANGLE"
	settingsOfTheGame   = "/game.project"
	sectionOfTheDisplay = "display"
	widthOfADisplay     = 960
	heightOfADisplay    = 640
	wholeTurn           = 360
	depthOfTheDisplay   = -1
	MostNodes           = MostObjects
)

var fieldsOfANode = map[string]string{
	"1": "position", "2": "rotation", "3": "scale", "4": "size", "5": "color", "6": "type", "8": "text", "9": "texture",
	"10": "font", "14": "pivot", "20": "layer", "21": "inherit_alpha", "22": "slice9", "23": "outerBounds", "24": "innerRadius",
	"26": "pieFillAngle", "30": "alpha", "38": "size_mode", "45": "enabled", "46": "visible",
}

var namesOfNodeTypes = map[string]string{
	typeOfABoxNode: "box", typeOfATextNode: "text", typeOfAPieNode: "pie", typeOfATemplateNode: "template",
	"TYPE_PARTICLEFX": "particlefx", "TYPE_CUSTOM": "custom",
}

var imagesTakenWhole = map[string]bool{".png": true, ".jpg": true, ".jpeg": true}

type guiNode struct {
	id        string
	parent    string
	scene     string
	own       *textproto.Message
	from      *guiNode
	overrides map[string]bool
	where     lines
}

func (n *guiNode) holderOf(field string) (holder *textproto.Message, scene string) {
	for n.from != nil && !n.overrides[field] {
		n = n.from
	}
	return n.own, n.scene
}

func (n *guiNode) text(field string) string {
	holder, _ := n.holderOf(field)
	return holder.Text(field)
}

func (n *guiNode) number(field string, otherwise float64) float64 {
	holder, _ := n.holderOf(field)
	return numberOf(holder, field, otherwise)
}

func (n *guiNode) isSet(field string, otherwise bool) bool {
	switch n.text(field) {
	case "true":
		return true
	case "false":
		return false
	}
	return otherwise
}

func (n *guiNode) vector(field string, otherwise float64) (x, y, z, w float64) {
	holder, _ := n.holderOf(field)
	written, isWritten := partOf(holder, field)
	if !isWritten {
		return otherwise, otherwise, otherwise, otherwise
	}
	return numberOf(written, "x", otherwise), numberOf(written, "y", otherwise), numberOf(written, "z", otherwise), numberOf(written, "w", otherwise)
}

func turnedBy(x, y, z float64) Transform {
	sinOf := func(degrees float64) float64 { return math.Sin(degrees * math.Pi / (wholeTurn / 2)) }
	cosOf := func(degrees float64) float64 { return math.Cos(degrees * math.Pi / (wholeTurn / 2)) }
	aboutX := Transform{{1, 0, 0, 0}, {0, cosOf(x), -sinOf(x), 0}, {0, sinOf(x), cosOf(x), 0}}
	aboutY := Transform{{cosOf(y), 0, sinOf(y), 0}, {0, 1, 0, 0}, {-sinOf(y), 0, cosOf(y), 0}}
	aboutZ := Transform{{cosOf(z), -sinOf(z), 0, 0}, {sinOf(z), cosOf(z), 0, 0}, {0, 0, 1, 0}}
	return aboutY.Times(aboutZ).Times(aboutX)
}

func (n *guiNode) placed() Transform {
	x, y, z, _ := n.vector("rotation", 0)
	placed := turnedBy(x, y, z)
	across, up, deep, _ := n.vector("scale", 1)
	for row := range 3 {
		placed[row][0], placed[row][1], placed[row][2] = placed[row][0]*across, placed[row][1]*up, placed[row][2]*deep
	}
	placed[0][3], placed[1][3], placed[2][3], _ = n.vector("position", 0)
	return placed
}

func (b *builder) nodesOf(resourcePath string, depth int) []*guiNode {
	if known, isKnown := b.nodes[resourcePath]; isKnown {
		return known
	}
	described, isRead := b.document(resourcePath)
	if !isRead || depth > deepestNesting {
		return nil
	}
	written := map[string]textproto.Field{}
	var placed []textproto.Field
	for _, field := range described.Fields {
		if field.Name != "nodes" || field.Message == nil {
			continue
		}
		written[field.Message.Text("id")] = field
		if field.Message.Text("template_node_child") != "true" {
			placed = append(placed, field)
		}
	}
	var nodes []*guiNode
	hasRoom := func() bool {
		if len(nodes) < MostNodes {
			return true
		}
		b.scene.IsCutShort = true
		return false
	}
	for _, field := range placed {
		if !hasRoom() {
			break
		}
		node := &guiNode{id: field.Message.Text("id"), parent: field.Message.Text("parent"), scene: resourcePath, own: field.Message, where: linesOf(field)}
		nodes = append(nodes, node)
		template := field.Message.Text("template")
		if field.Message.Text("type") != typeOfATemplateNode || b.isHolding[template] {
			continue
		}
		b.isHolding[template] = true
		inside := b.nodesOf(template, depth+1)
		delete(b.isHolding, template)
		for _, inner := range inside {
			if !hasRoom() {
				break
			}
			child := &guiNode{id: node.id + separatorOfIDs + inner.id, parent: node.id, scene: resourcePath, own: &textproto.Message{}, from: inner, where: node.where}
			if inner.parent != "" {
				child.parent = node.id + separatorOfIDs + inner.parent
			}
			if entry, isWritten := written[child.id]; isWritten && entry.Message.Text("template_node_child") == "true" {
				child.own, child.where, child.overrides = entry.Message, linesOf(entry), map[string]bool{}
				for _, number := range entry.Message.Scalars("overridden_fields") {
					child.overrides[fieldsOfANode[number.Text]] = true
				}
			}
			nodes = append(nodes, child)
		}
	}
	b.nodes[resourcePath] = nodes
	return nodes
}

func (b *builder) namedIn(scenePath, list, name, field string) string {
	described, isRead := b.document(scenePath)
	if !isRead {
		return ""
	}
	for _, each := range described.Messages(list) {
		if each.Text("name") == name {
			return each.Text(field)
		}
	}
	return ""
}

func (b *builder) displayOfTheGame() (width, height float64) {
	width, height = widthOfADisplay, heightOfADisplay
	content, err := b.content(settingsOfTheGame)
	if err != nil {
		return width, height
	}
	section := ""
	lines := bufio.NewScanner(bytes.NewReader(content))
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, isSetting := strings.Cut(line, "=")
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if !isSetting || section != sectionOfTheDisplay || err != nil || number <= 0 {
			continue
		}
		switch strings.TrimSpace(key) {
		case "width":
			width = number
		case "height":
			height = number
		}
	}
	return width, height
}

func tintOf(red, green, blue float64) string {
	part := func(share float64) int { return int(math.Round(255 * min(1, max(0, share)))) }
	return fmt.Sprintf("#%02x%02x%02x", part(red), part(green), part(blue))
}

type shownNode struct {
	world Transform
	alpha float64
	layer string
	isOff bool
}

func (b *builder) node(node *guiNode, parent shownNode, depthOf map[string]float64) shownNode {
	shown := shownNode{world: parent.world.Times(node.placed()), layer: parent.layer, isOff: parent.isOff || !node.isSet("enabled", true)}
	if layer := node.text("layer"); layer != "" {
		shown.layer = layer
	}
	red, green, blue, opacity := node.vector("color", 1)
	shown.alpha = node.number("alpha", opacity)
	if node.isSet("inherit_alpha", false) {
		shown.alpha *= parent.alpha
	}
	kind := node.text("type")
	if kind == "" {
		kind = typeOfABoxNode
	}
	if kind == typeOfATemplateNode {
		return shown
	}
	width, height, _, _ := node.vector("size", 0)
	item := Item{
		Owner: node.id, Type: namesOfNodeTypes[kind], Tint: tintOf(red, green, blue), Layer: shown.layer,
		IsOff: shown.isOff || !node.isSet("visible", true), Width: width, Height: height,
	}
	var offX, offY float64
	switch kind {
	case typeOfABoxNode:
		item.Kind = KindOfAPanel
		textureHolder, sceneOfTheTexture := node.holderOf("texture")
		name, animation, _ := strings.Cut(textureHolder.Text("texture"), separatorOfIDs)
		if texture := b.namedIn(sceneOfTheTexture, "textures", name, "texture"); texture != "" {
			if found := b.pictureOf(texture, animation); found.isThere {
				item.Kind, item.Image, item.From, item.Flip = KindOfASprite, found.index, found.from, found.flip
				if node.text("size_mode") == sizeTakenFromImage {
					item.Width, item.Height = float64(found.from[2]), float64(found.from[3])
				}
				item.Slice9 = edgesKept(node.vector("slice9", 0))
				offX, offY = found.offX, found.offY
			}
		}
	case typeOfATextNode:
		item.Kind, item.Text = KindOfALabel, node.text("text")
		fontHolder, sceneOfTheFont := node.holderOf("font")
		if font := b.namedIn(sceneOfTheFont, "fonts", fontHolder.Text("font"), "font"); font != "" {
			if described, isRead := b.document(font); isRead {
				item.TextSize = numberOf(described, "size", 0)
			}
		}
	case typeOfAPieNode:
		item.Kind, item.Inner, item.Sweep = KindOfAPie, node.number("innerRadius", 0), node.number("pieFillAngle", wholeTurn)
		item.IsSquare = node.text("outerBounds") == boundsOfASquarePie
	default:
		item.Kind = KindOfAMarker
	}
	pivot := pivotsOfALabel[node.text("pivot")]
	if item.Kind == KindOfALabel {
		item.LeanX, item.LeanY = sideOf(pivot[0]), sideOf(pivot[1])
	}
	placed := shown.world
	if item.Kind != KindOfAMarker {
		placed = placed.Times(moved((pivot[0]+offX)*item.Width, (pivot[1]+offY)*item.Height))
		b.reach(placed, item.Width/2, item.Height/2)
	} else {
		b.reach(placed, 0, 0)
	}
	b.add(item, placed, node.where)
	last := &b.scene.Items[len(b.scene.Items)-1]
	last.Alpha, last.Depth = shown.alpha, depthOf[shown.layer]
	return shown
}

func sideOf(offsetOfTheMiddle float64) int {
	switch {
	case offsetOfTheMiddle > 0:
		return -1
	case offsetOfTheMiddle < 0:
		return 1
	}
	return 0
}

func edgesKept(left, top, right, bottom float64) []float64 {
	if left <= 0 && top <= 0 && right <= 0 && bottom <= 0 {
		return nil
	}
	return []float64{left, top, right, bottom}
}

func (b *builder) gui(resourcePath string, described *textproto.Message) {
	nodes := b.nodesOf(resourcePath, 0)
	depthOf := map[string]float64{}
	for index, layer := range described.Messages("layers") {
		depthOf[layer.Text("name")] = float64(index + 1)
	}
	isThere := map[string]bool{}
	childrenOf := map[string][]*guiNode{}
	for _, node := range nodes {
		isThere[node.id] = true
	}
	for _, node := range nodes {
		parent := node.parent
		if !isThere[parent] {
			parent = ""
		}
		childrenOf[parent] = append(childrenOf[parent], node)
	}
	isPlaced := map[*guiNode]bool{}
	var place func(under string, parent shownNode)
	place = func(under string, parent shownNode) {
		for _, node := range childrenOf[under] {
			if isPlaced[node] {
				continue
			}
			isPlaced[node] = true
			b.scene.Objects++
			shown := b.node(node, parent, depthOf)
			if node.id != "" {
				place(node.id, shown)
			}
		}
	}
	place("", shownNode{world: identity(), alpha: 1})

	width, height := b.displayOfTheGame()
	b.scene.Display = &[2]float64{width, height}
	display := moved(width/2, height/2)
	b.reach(display, width/2, height/2)
	b.add(Item{Kind: KindOfTheDisplay, Type: KindOfTheDisplay, Width: width, Height: height}, display, lines{})
	b.scene.Items[len(b.scene.Items)-1].Depth = depthOfTheDisplay
}

func isAnImageTakenWhole(resourcePath string) bool {
	return imagesTakenWhole[strings.ToLower(path.Ext(resourcePath))]
}
