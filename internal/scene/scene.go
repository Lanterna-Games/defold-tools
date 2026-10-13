package scene

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"maps"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
	"github.com/Lanterna-Games/defold-tools/internal/tiles"
)

const (
	ExtensionOfAGameObject = ".go"
	ExtensionOfACollection = ".collection"

	KindOfASprite  = "sprite"
	KindOfAMap     = "tilemap"
	KindOfALabel   = "label"
	KindOfABox     = "box"
	KindOfASphere  = "sphere"
	KindOfACapsule = "capsule"
	KindOfAMarker  = "marker"

	FlippedAcross   = 1
	FlippedUpDown   = 2
	middleOfAnImage = 0.5

	typeOfACollisionObject = "collisionobject"
	deepestNesting         = 16
	MostObjects            = 20000
	MostCells              = 1000000
	startOfAnImageInline   = "data:%s;base64,%s"
	sizeSetByHand          = "SIZE_MODE_MANUAL"
	shapeOfABox            = "TYPE_BOX"
	shapeOfASphere         = "TYPE_SPHERE"
	shapeOfACapsule        = "TYPE_CAPSULE"
	separatorOfIDs         = "/"
)

var ErrNotAGameObject = errors.New("this is not a game object of Defold")

type Files struct {
	All    fs.FS
	OnDisk fs.FS
	Held   map[string][]byte
}

type Image struct {
	Path string `json:"path"`
	Data string `json:"data,omitempty"`
}

type PlacedMap struct {
	Image      int      `json:"image"`
	TileWidth  int      `json:"tileWidth"`
	TileHeight int      `json:"tileHeight"`
	Tiles      [][2]int `json:"tiles"`
	Cells      [][4]int `json:"cells"`

	minX int
	maxX int
	minY int
	maxY int
}

type Item struct {
	Kind      string     `json:"kind"`
	Owner     string     `json:"owner"`
	Component string     `json:"component"`
	Type      string     `json:"type"`
	Across    [6]float64 `json:"across"`
	Depth     float64    `json:"depth"`
	Line      int        `json:"line"`
	LastLine  int        `json:"lastLine"`
	Image     int        `json:"image"`
	From      [4]int     `json:"from"`
	Width     float64    `json:"width"`
	Height    float64    `json:"height"`
	Radius    float64    `json:"radius,omitempty"`
	Text      string     `json:"text,omitempty"`
	Map       int        `json:"map"`
	Alpha     float64    `json:"alpha"`
	Tint      string     `json:"tint,omitempty"`
	Layer     string     `json:"layer,omitempty"`
	IsOff     bool       `json:"off,omitempty"`
	Slice9    []float64  `json:"slice9,omitempty"`
	TextSize  float64    `json:"textSize,omitempty"`
	LeanX     int        `json:"leanX,omitempty"`
	LeanY     int        `json:"leanY,omitempty"`
	Inner     float64    `json:"inner,omitempty"`
	Sweep     float64    `json:"sweep,omitempty"`
	IsSquare  bool       `json:"square,omitempty"`
	Flip      int        `json:"flip,omitempty"`
}

type Scene struct {
	Images     []Image      `json:"images"`
	Maps       []*PlacedMap `json:"maps"`
	Items      []Item       `json:"items"`
	Uses       []string     `json:"uses"`
	Objects    int          `json:"objects"`
	Display    *[2]float64  `json:"display,omitempty"`
	IsCutShort bool         `json:"cutShort"`
	MinX       float64      `json:"minX"`
	MinY       float64      `json:"minY"`
	MaxX       float64      `json:"maxX"`
	MaxY       float64      `json:"maxY"`

	isBounded bool
}

type lines struct {
	first int
	last  int
}

type picture struct {
	index   int
	from    [4]int
	isThere bool
	flip    int
	offX    float64
	offY    float64
}

type builder struct {
	files     Files
	scene     *Scene
	indexOf   map[string]int
	sizeOf    map[string]image.Config
	parsed    map[string]*textproto.Message
	isUsed    map[string]bool
	mapOf     map[string]int
	pictures  map[[2]string]picture
	isHolding map[string]bool
	embedded  map[*textproto.Scalar]*textproto.Message
	nodes     map[string][]*guiNode
	cells     int
}

func (b *builder) embeddedIn(data *textproto.Scalar) *textproto.Message {
	if parsed, isParsed := b.embedded[data]; isParsed {
		return parsed
	}
	parsed, _ := data.Embedded()
	b.embedded[data] = parsed
	return parsed
}

func (b *builder) hasRoom() bool {
	if b.scene.Objects < MostObjects && b.cells < MostCells {
		return true
	}
	b.scene.IsCutShort = true
	return false
}

func lastLineOf(field textproto.Field) int {
	last := field.Position.Line
	if field.Message == nil {
		return last
	}
	for _, inner := range field.Message.Fields {
		last = max(last, lastLineOf(inner))
	}
	return last + 1
}

func linesOf(field textproto.Field) lines {
	return lines{first: field.Position.Line, last: lastLineOf(field)}
}

func (b *builder) content(resourcePath string) ([]byte, error) {
	name := strings.TrimPrefix(resourcePath, "/")
	b.isUsed[resourcePath] = true
	if held, isHeld := b.files.Held[name]; isHeld {
		return held, nil
	}
	return fs.ReadFile(b.files.All, name)
}

func (b *builder) document(resourcePath string) (*textproto.Message, bool) {
	if parsed, isParsed := b.parsed[resourcePath]; isParsed {
		return parsed, parsed != nil
	}
	content, err := b.content(resourcePath)
	if err != nil {
		b.parsed[resourcePath] = nil
		return nil, false
	}
	parsed, _ := textproto.Parse(content, source.NewLocator(content))
	b.parsed[resourcePath] = parsed
	return parsed, true
}

func (b *builder) image(resourcePath string) (index int, size image.Config, isThere bool) {
	if index, isKnown := b.indexOf[resourcePath]; isKnown {
		return index, b.sizeOf[resourcePath], index >= 0
	}
	b.indexOf[resourcePath] = -1
	b.isUsed[resourcePath] = true
	name := strings.TrimPrefix(resourcePath, "/")
	content, err := fs.ReadFile(b.files.All, name)
	if err != nil {
		return -1, image.Config{}, false
	}
	size, _, err = image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return -1, image.Config{}, false
	}
	shown := Image{Path: resourcePath}
	if _, err := fs.Stat(b.files.OnDisk, name); err != nil {
		shown.Data = fmt.Sprintf(startOfAnImageInline, http.DetectContentType(content), base64.StdEncoding.EncodeToString(content))
	}
	b.scene.Images = append(b.scene.Images, shown)
	b.indexOf[resourcePath], b.sizeOf[resourcePath] = len(b.scene.Images)-1, size
	return len(b.scene.Images) - 1, size, true
}

func (b *builder) reach(placed Transform, halfWidth, halfHeight float64) {
	for _, corner := range [][2]float64{{-halfWidth, -halfHeight}, {halfWidth, -halfHeight}, {halfWidth, halfHeight}, {-halfWidth, halfHeight}} {
		x, y := placed.At(corner[0], corner[1])
		if !b.scene.isBounded {
			b.scene.MinX, b.scene.MaxX, b.scene.MinY, b.scene.MaxY, b.scene.isBounded = x, x, y, y, true
		}
		b.scene.MinX, b.scene.MaxX = min(b.scene.MinX, x), max(b.scene.MaxX, x)
		b.scene.MinY, b.scene.MaxY = min(b.scene.MinY, y), max(b.scene.MaxY, y)
	}
}

func (b *builder) add(item Item, placed Transform, where lines) {
	item.Across, item.Depth, item.Line, item.LastLine, item.Alpha = placed.Across(), placed.Depth(), where.first, where.last, 1
	if item.Kind != KindOfASprite && item.Kind != KindOfAMap {
		item.Image = -1
	}
	if item.Kind != KindOfAMap {
		item.Map = -1
	}
	b.scene.Items = append(b.scene.Items, item)
}

func moved(x, y float64) Transform {
	shift := identity()
	shift[0][3], shift[1][3] = x, y
	return shift
}

func (b *builder) pictureOf(texture, animation string) picture {
	key := [2]string{texture, animation}
	if found, isKnown := b.pictures[key]; isKnown {
		return found
	}
	found := b.pictureIn(texture, animation)
	b.pictures[key] = found
	return found
}

func flipOf(isAcross, isUpDown bool) int {
	return boolToInt(isAcross)*FlippedAcross | boolToInt(isUpDown)*FlippedUpDown
}

func (b *builder) pictureIn(texture, animation string) picture {
	if isAnImageTakenWhole(texture) {
		index, size, isThere := b.image(texture)
		return picture{index: index, from: [4]int{0, 0, size.Width, size.Height}, isThere: isThere}
	}
	described, isRead := b.document(texture)
	if !isRead {
		return picture{index: -1}
	}
	if path.Ext(texture) == tiles.SourceExtension {
		content, err := b.content(texture)
		if err != nil {
			return picture{index: -1}
		}
		read := tiles.SourceOf(content)
		index, size, isThere := b.image(read.Image)
		grid, err := read.GridOver(size.Width, size.Height)
		if !isThere || err != nil {
			return picture{index: -1}
		}
		shown := picture{index: index, isThere: true}
		tile := 0
		for _, each := range read.Animations {
			if each.ID == animation && each.First >= 0 && each.First < len(grid.Tiles) {
				tile, shown.flip = each.First, flipOf(each.IsFlippedAcross, each.IsFlippedUpDown)
			}
		}
		shown.from = [4]int{grid.Tiles[tile].X, grid.Tiles[tile].Y, read.TileWidth, read.TileHeight}
		return shown
	}
	patterns := described.Text("rename_patterns")
	var named *textproto.Message
	shown := picture{}
	for _, each := range described.Messages("images") {
		if atlas.NameOfAnImage(each.Text("image"), patterns) == animation {
			named = each
		}
	}
	for _, each := range described.Messages("animations") {
		if frames := each.Messages("images"); each.Text("id") == animation && len(frames) > 0 {
			named, shown.flip = frames[0], flipOf(numberOf(each, "flip_horizontal", 0) != 0, numberOf(each, "flip_vertical", 0) != 0)
		}
	}
	if named == nil {
		return picture{index: -1}
	}
	index, size, isThere := b.image(named.Text("image"))
	shown.index, shown.from, shown.isThere = index, [4]int{0, 0, size.Width, size.Height}, isThere
	shown.offX, shown.offY = middleOfAnImage-numberOf(named, "pivot_x", middleOfAnImage), numberOf(named, "pivot_y", middleOfAnImage)-middleOfAnImage
	return shown
}

func (b *builder) sprite(described *textproto.Message, item Item, placed Transform, where lines) {
	texture := described.Text("tile_set")
	if textures := described.Messages("textures"); len(textures) > 0 {
		texture = textures[0].Text("texture")
	}
	shown := b.pictureOf(texture, described.Text("default_animation"))
	if !shown.isThere {
		b.marker(item, placed, where)
		return
	}
	item.Kind, item.Image, item.From, item.Flip = KindOfASprite, shown.index, shown.from, shown.flip
	item.Width, item.Height = float64(shown.from[2]), float64(shown.from[3])
	if size, isWritten := partOf(described, "size"); isWritten && described.Text("size_mode") == sizeSetByHand {
		item.Width, item.Height = numberOf(size, "x", item.Width), numberOf(size, "y", item.Height)
		if edges, isSliced := partOf(described, "slice9"); isSliced {
			item.Slice9 = edgesKept(numberOf(edges, "x", 0), numberOf(edges, "y", 0), numberOf(edges, "z", 0), numberOf(edges, "w", 0))
		}
	}
	placed = placed.Times(moved(shown.offX*item.Width, shown.offY*item.Height))
	b.reach(placed, item.Width/2, item.Height/2)
	b.add(item, placed, where)
}

var pivotsOfALabel = map[string][2]float64{
	"PIVOT_CENTER": {0, 0}, "PIVOT_N": {0, -0.5}, "PIVOT_NE": {-0.5, -0.5}, "PIVOT_E": {-0.5, 0}, "PIVOT_SE": {-0.5, 0.5},
	"PIVOT_S": {0, 0.5}, "PIVOT_SW": {0.5, 0.5}, "PIVOT_W": {0.5, 0}, "PIVOT_NW": {0.5, -0.5},
}

func (b *builder) label(described *textproto.Message, item Item, placed Transform, where lines) {
	item.Kind, item.Text = KindOfALabel, described.Text("text")
	if size, isWritten := partOf(described, "size"); isWritten {
		item.Width, item.Height = numberOf(size, "x", 0), numberOf(size, "y", 0)
	}
	pivot := pivotsOfALabel[described.Text("pivot")]
	placed = placed.Times(moved(pivot[0]*item.Width, pivot[1]*item.Height))
	b.reach(placed, item.Width/2, item.Height/2)
	b.add(item, placed, where)
}

func (b *builder) shapes(described *textproto.Message, item Item, placed Transform, where lines) {
	embedded, isWritten := partOf(described, "embedded_collision_shape")
	if !isWritten {
		b.marker(item, placed, where)
		return
	}
	var data []float64
	for _, written := range embedded.Scalars("data") {
		number, _ := strconv.ParseFloat(written.Text, 64)
		data = append(data, number)
	}
	drawn := 0
	for _, shape := range embedded.Messages("shapes") {
		first := int(numberOf(shape, "index", 0))
		of := func(offset int) float64 {
			if first+offset < 0 || first+offset >= len(data) {
				return 0
			}
			return data[first+offset]
		}
		outlined, at := item, placed.Times(placedBy(shape))
		switch shape.Text("shape_type") {
		case shapeOfABox:
			outlined.Kind, outlined.Width, outlined.Height = KindOfABox, 2*of(0), 2*of(1)
		case shapeOfASphere:
			outlined.Kind, outlined.Radius = KindOfASphere, of(0)
			outlined.Width, outlined.Height = 2*of(0), 2*of(0)
		case shapeOfACapsule:
			outlined.Kind, outlined.Radius = KindOfACapsule, of(0)
			outlined.Width, outlined.Height = 2*of(0), of(1)+2*of(0)
		default:
			continue
		}
		b.reach(at, outlined.Width/2, outlined.Height/2)
		b.add(outlined, at, where)
		drawn++
	}
	if drawn == 0 {
		b.marker(item, placed, where)
	}
}

func (b *builder) mapToPlace(key string, content []byte) (index int, isThere bool) {
	if index, isKnown := b.mapOf[key]; isKnown {
		return index, index >= 0
	}
	b.mapOf[key] = -1
	read := tiles.MapOf(content)
	sourceContent, err := b.content(read.TileSource)
	if err != nil {
		return -1, false
	}
	cut := tiles.SourceOf(sourceContent)
	picture, size, isThere := b.image(cut.Image)
	grid, err := cut.GridOver(size.Width, size.Height)
	if !isThere || err != nil {
		return -1, false
	}
	layers := slices.Clone(read.Layers)
	slices.SortStableFunc(layers, func(first, second tiles.Layer) int { return cmp.Compare(first.Z, second.Z) })
	drawn := &PlacedMap{Image: picture, TileWidth: cut.TileWidth, TileHeight: cut.TileHeight, Cells: [][4]int{}, Tiles: make([][2]int, 0, len(grid.Tiles))}
	for _, tile := range grid.Tiles {
		drawn.Tiles = append(drawn.Tiles, [2]int{tile.X, tile.Y})
	}
	for _, layer := range layers {
		if !layer.IsVisible {
			continue
		}
		for _, cell := range layer.Cells {
			if cell.Tile < 0 || cell.Tile >= len(grid.Tiles) {
				continue
			}
			done := boolToInt(cell.IsFlippedAcross) | boolToInt(cell.IsFlippedUpDown)<<1 | boolToInt(cell.IsTurned)<<2
			drawn.Cells = append(drawn.Cells, [4]int{cell.X, cell.Y, cell.Tile, done})
			if len(drawn.Cells) == 1 {
				drawn.minX, drawn.maxX, drawn.minY, drawn.maxY = cell.X, cell.X, cell.Y, cell.Y
			}
			drawn.minX, drawn.maxX = min(drawn.minX, cell.X), max(drawn.maxX, cell.X)
			drawn.minY, drawn.maxY = min(drawn.minY, cell.Y), max(drawn.maxY, cell.Y)
		}
	}
	b.scene.Maps = append(b.scene.Maps, drawn)
	b.mapOf[key] = len(b.scene.Maps) - 1
	return len(b.scene.Maps) - 1, true
}

func (b *builder) tileMap(key string, content []byte, item Item, placed Transform, where lines) {
	index, isThere := b.mapToPlace(key, content)
	if !isThere {
		b.marker(item, placed, where)
		return
	}
	drawn := b.scene.Maps[index]
	b.cells += len(drawn.Cells)
	if len(drawn.Cells) > 0 {
		wide, high := float64(drawn.TileWidth), float64(drawn.TileHeight)
		reached := placed.Times(moved(float64(drawn.minX+drawn.maxX+1)*wide/2, float64(drawn.minY+drawn.maxY+1)*high/2))
		b.reach(reached, float64(drawn.maxX-drawn.minX+1)*wide/2, float64(drawn.maxY-drawn.minY+1)*high/2)
	}
	item.Kind, item.Map, item.Image = KindOfAMap, index, drawn.Image
	b.add(item, placed, where)
}

func boolToInt(isSo bool) int {
	if isSo {
		return 1
	}
	return 0
}

func (b *builder) marker(item Item, placed Transform, where lines) {
	item.Kind = KindOfAMarker
	b.reach(placed, 0, 0)
	b.add(item, placed, where)
}

func (b *builder) component(kind string, described *textproto.Message, key string, content []byte, item Item, placed Transform, where lines) {
	item.Type = kind
	switch {
	case described == nil:
		b.marker(item, placed, where)
	case kind == KindOfASprite:
		b.sprite(described, item, placed, where)
	case kind == KindOfALabel:
		b.label(described, item, placed, where)
	case kind == typeOfACollisionObject:
		b.shapes(described, item, placed, where)
	case kind == KindOfAMap:
		b.tileMap(key, content, item, placed, where)
	default:
		b.marker(item, placed, where)
	}
}

func (b *builder) gameObject(described *textproto.Message, world Transform, owner string, whereOf func(textproto.Field) lines) {
	b.scene.Objects++
	hasComponents := false
	for _, field := range described.Fields {
		if field.Message == nil {
			continue
		}
		item := Item{Owner: owner, Component: field.Message.Text("id")}
		placed, where := world.Times(placedBy(field.Message)), whereOf(field)
		switch field.Name {
		case "components":
			resourcePath := field.Message.Text("component")
			kind := strings.TrimPrefix(path.Ext(resourcePath), ".")
			file := &textproto.Message{}
			var content []byte
			switch _, isPlaced := b.mapOf[resourcePath]; {
			case kind == KindOfAMap && !isPlaced:
				content, _ = b.content(resourcePath)
			case kind == KindOfASprite, kind == KindOfALabel, kind == typeOfACollisionObject:
				file, _ = b.document(resourcePath)
			}
			b.component(kind, file, resourcePath, content, item, placed, where)
		case "embedded_components":
			data, isWritten := field.Message.Scalar("data")
			if !isWritten {
				continue
			}
			b.component(field.Message.Text("type"), b.embeddedIn(data), data.Text, []byte(data.Text), item, placed, where)
		default:
			continue
		}
		hasComponents = true
	}
	if !hasComponents {
		b.marker(Item{Owner: owner, Type: "empty"}, world, whereOf(textproto.Field{}))
	}
}

func (b *builder) collection(described *textproto.Message, world Transform, prefix string, depth int, whereOf func(textproto.Field) lines) {
	if depth > deepestNesting {
		return
	}
	type instance struct {
		field textproto.Field
		local Transform
	}
	instances := map[string]instance{}
	var order []string
	isAChild := map[string]bool{}
	for _, field := range described.Fields {
		if field.Message == nil || (field.Name != "instances" && field.Name != "embedded_instances") {
			continue
		}
		id := field.Message.Text("id")
		instances[id] = instance{field: field, local: placedBy(field.Message)}
		order = append(order, id)
		for _, child := range field.Message.Scalars("children") {
			isAChild[child.Text] = true
		}
	}
	var place func(id string, parent Transform, isSeen map[string]bool)
	place = func(id string, parent Transform, isSeen map[string]bool) {
		placed, isThere := instances[id]
		if !isThere || isSeen[id] || !b.hasRoom() {
			return
		}
		isSeen[id] = true
		world := parent.Times(placed.local)
		where := whereOf(placed.field)
		fixed := func(textproto.Field) lines { return where }
		owner := prefix + separatorOfIDs + id
		if placed.field.Name == "instances" {
			if prototype, isRead := b.document(placed.field.Message.Text("prototype")); isRead {
				b.gameObject(prototype, world, owner, fixed)
			} else {
				b.scene.Objects++
				b.marker(Item{Owner: owner, Type: "missing"}, world, where)
			}
		} else if data, isWritten := placed.field.Message.Scalar("data"); isWritten {
			b.gameObject(b.embeddedIn(data), world, owner, fixed)
		}
		for _, child := range placed.field.Message.Scalars("children") {
			place(child.Text, world, isSeen)
		}
	}
	isSeen := map[string]bool{}
	for _, id := range order {
		if !isAChild[id] {
			place(id, world, isSeen)
		}
	}
	for _, field := range described.Fields {
		if field.Message == nil || field.Name != "collection_instances" {
			continue
		}
		resourcePath := field.Message.Text("collection")
		nested, isRead := b.document(resourcePath)
		if !isRead || b.isHolding[resourcePath] || !b.hasRoom() {
			continue
		}
		where := whereOf(field)
		b.isHolding[resourcePath] = true
		b.collection(nested, world.Times(placedBy(field.Message)), prefix+separatorOfIDs+field.Message.Text("id"), depth+1, func(textproto.Field) lines { return where })
		delete(b.isHolding, resourcePath)
	}
}

func Of(files Files, resourcePath string) (*Scene, error) {
	built := &builder{
		files: files, scene: &Scene{Images: []Image{}, Maps: []*PlacedMap{}, Items: []Item{}},
		indexOf: map[string]int{}, sizeOf: map[string]image.Config{}, parsed: map[string]*textproto.Message{}, isUsed: map[string]bool{},
		mapOf: map[string]int{}, pictures: map[[2]string]picture{}, isHolding: map[string]bool{resourcePath: true},
		embedded: map[*textproto.Scalar]*textproto.Message{}, nodes: map[string][]*guiNode{},
	}
	content, err := built.content(resourcePath)
	if err != nil {
		return nil, fmt.Errorf("%s is not in the project", resourcePath)
	}
	described, mistakes := textproto.Parse(content, source.NewLocator(content))
	built.parsed[resourcePath] = described
	switch path.Ext(resourcePath) {
	case ExtensionOfAGameObject:
		if len(mistakes) > 0 && len(described.Messages("components"))+len(described.Messages("embedded_components")) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrNotAGameObject, resourcePath)
		}
		built.gameObject(described, identity(), "", linesOf)
	case ExtensionOfACollection:
		built.collection(described, identity(), "", 0, linesOf)
	case ExtensionOfAGUI:
		built.gui(resourcePath, described)
	default:
		return nil, fmt.Errorf("%s is not a game object, a collection or a GUI scene", resourcePath)
	}
	slices.SortStableFunc(built.scene.Items, func(first, second Item) int { return cmp.Compare(first.Depth, second.Depth) })
	delete(built.isUsed, resourcePath)
	delete(built.isUsed, "")
	built.scene.Uses = slices.Sorted(maps.Keys(built.isUsed))
	return built.scene, nil
}
