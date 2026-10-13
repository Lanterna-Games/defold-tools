package scene

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func studyGUI(t *testing.T) fstest.MapFS {
	t.Helper()
	project := studyProject(t)
	project["main/wide.png"] = &fstest.MapFile{Data: pictureOf(t, 48, 16)}
	project["main/tall.png"] = &fstest.MapFile{Data: pictureOf(t, 16, 40)}
	return project
}

func nodeOf(t *testing.T, laidOut *Scene, id string) Item {
	t.Helper()
	for _, item := range laidOut.Items {
		if item.Owner == id {
			return item
		}
	}
	t.Fatalf("no node %q among %d items", id, len(laidOut.Items))
	return Item{}
}

// What the engine does in gui.pick_node, over the same points the study asked it
// about: every second pixel of a display of 960 by 640, from 1.
func pointsOver(item Item) (count int, centreX, centreY float64) {
	a, b, c, d, x, y := item.Across[0], item.Across[1], item.Across[2], item.Across[3], item.Across[4], item.Across[5]
	turn := a*d - b*c
	if turn == 0 {
		return 0, 0, 0
	}
	for row := 1; row <= 640; row += 2 {
		for column := 1; column <= 960; column += 2 {
			across := (d*(float64(column)-x) - c*(float64(row)-y)) / turn
			up := (-b*(float64(column)-x) + a*(float64(row)-y)) / turn
			if math.Abs(across) <= item.Width/2 && math.Abs(up) <= item.Height/2 {
				count, centreX, centreY = count+1, centreX+float64(column), centreY+float64(row)
			}
		}
	}
	if count > 0 {
		centreX, centreY = centreX/float64(count), centreY/float64(count)
	}
	return count, centreX, centreY
}

// The numbers are the engine's: testdata/README.md has what it printed and how
// it was asked. Where a node stands is where its pivot is, and the points it
// covers say where its box lies around that. The engine turns a node with a
// table of sines that is good to a thousandth, so a node under a turn that is
// not a multiple of 45 degrees lands a fraction of a pixel from where exact
// sines put it; those rows allow a fifth of a pixel.
func TestNodesOfAGUIStandWhereTheEnginePutsThem(t *testing.T) {
	const exactly, closely = 0.001, 0.2
	cases := []struct {
		id      string
		pivot   string
		x, y    float64
		wide    float64
		high    float64
		covered int
		centreX float64
		centreY float64
		within  float64
	}{
		{"plain", "PIVOT_CENTER", 100, 50, 200, 100, 5000, 100, 50, exactly},
		{"turned", "PIVOT_SW", 400, 300, 100, 60, 1342, 475.346, 362.595, exactly},
		{"turned_child", "PIVOT_CENTER", 477.5058, 361.9161, 10, 10, 22, 477.455, 361.909, closely},
		{"turned_flat_child", "PIVOT_CENTER", 475.4194, 362.6671, 10, 10, 22, 475.273, 362.545, closely},
		{"auto", "PIVOT_CENTER", 700, 500, 48, 16, 192, 700, 500, exactly},
		{"stretched", "PIVOT_NE", 800, 100, 90, 30, 676, 755.846, 117.464, exactly},
		{"no_size", "PIVOT_CENTER", 10, 10, 0, 0, 0, 0, 0, exactly},
		{"text", "PIVOT_NW", 300, 100, 200, 50, 2500, 400, 75, exactly},
		{"pie", "PIVOT_E", 150, 400, 120, 80, 2400, 90, 400, exactly},
		{"holder/root", "PIVOT_CENTER", 560, 220, 50, 50, 2500, 560, 220, exactly},
		{"holder/leaf", "PIVOT_CENTER", 542, 234, 20, 20, 400, 542, 234, exactly},
		{"holder/silent", "PIVOT_CENTER", 480, 220, 20, 20, 400, 480, 220, exactly},
		{"holder/nest/dot", "PIVOT_CENTER", 536, 182, 16, 4, 64, 536, 182, exactly},
		{"spun", "PIVOT_CENTER", 200, 550, 100, 60, 1346, 200, 550, exactly},
		{"spun_child", "PIVOT_CENTER", 230.7436, 599.7850, 10, 10, 23, 230.739, 599.783, closely},
		{"spun_flat_child", "PIVOT_CENTER", 228.6573, 600.5359, 10, 10, 21, 228.714, 600.619, closely},
		{"z30_child", "PIVOT_CENTER", 566.6067, 369.9798, 2, 2, 1, 567, 371, closely},
		{"z45_child", "PIVOT_CENTER", 550.7107, 390.7107, 2, 2, 1, 551, 391, exactly},
		{"z123_child", "PIVOT_CENTER", 424.9593, 403.4789, 2, 2, 1, 425, 403, closely},
	}
	laidOut := sceneOf(t, studyGUI(t), "/main/study.gui")
	for _, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			item := nodeOf(t, laidOut, testCase.id)
			if item.Width != testCase.wide || item.Height != testCase.high {
				t.Errorf("size = %v by %v, the engine says %v by %v", item.Width, item.Height, testCase.wide, testCase.high)
			}
			pivot := pivotsOfALabel[testCase.pivot]
			a, b, c, d := item.Across[0], item.Across[1], item.Across[2], item.Across[3]
			x := item.Across[4] - a*pivot[0]*item.Width - c*pivot[1]*item.Height
			y := item.Across[5] - b*pivot[0]*item.Width - d*pivot[1]*item.Height
			if math.Abs(x-testCase.x) > testCase.within || math.Abs(y-testCase.y) > testCase.within {
				t.Errorf("it stands at %.4f, %.4f, the engine says %.4f, %.4f", x, y, testCase.x, testCase.y)
			}
			covered, centreX, centreY := pointsOver(item)
			allowed := max(3, testCase.covered/100)
			if covered < testCase.covered-allowed || covered > testCase.covered+allowed {
				t.Errorf("it covers %d of the points asked, the engine says %d", covered, testCase.covered)
			}
			if testCase.covered > 20 && (math.Abs(centreX-testCase.centreX) > 0.6 || math.Abs(centreY-testCase.centreY) > 0.6) {
				t.Errorf("the points it covers centre on %.3f, %.3f, the engine says %.3f, %.3f", centreX, centreY, testCase.centreX, testCase.centreY)
			}
		})
	}
}

// A node that comes from a template is written again in the scene that holds
// the template, with the numbers of the fields it changes. A value written
// there for a field that is not among them is what the editor last saw in the
// template, and the build does not use it: "holder/silent" has a position of
// 999, 999 written and stands where the template puts it.
func TestANodeOfATemplatePointsAtWhereItIsWritten(t *testing.T) {
	project := studyGUI(t)
	written := string(project["main/study.gui"].Data)
	lineOf := func(text string) int {
		before, _, isWritten := strings.Cut(written, text)
		if !isWritten {
			t.Fatalf("the fixture does not write %q", text)
		}
		return strings.Count(before, "\n") + 1
	}

	laidOut := sceneOf(t, project, "/main/study.gui")

	leaf := nodeOf(t, laidOut, "holder/leaf")
	if firstLine := lineOf(`  id: "holder/leaf"`); leaf.Line > firstLine || leaf.LastLine < firstLine {
		t.Errorf("holder/leaf points at lines %d to %d, and is written around line %d", leaf.Line, leaf.LastLine, firstLine)
	}
	for _, item := range laidOut.Items {
		if item.Owner == "holder" || item.Owner == "holder/nest" {
			t.Errorf("the template %q is drawn as a %s: it only places what it holds", item.Owner, item.Kind)
		}
	}

	start := strings.Index(written, "nodes {\n  type: TYPE_BOX\n  id: \"holder/root\"")
	end := start + strings.Index(written[start:], "\n}\n") + len("\n}\n")
	project["main/study.gui"] = &fstest.MapFile{Data: []byte(written[:start] + written[end:])}
	root := nodeOf(t, sceneOf(t, project, "/main/study.gui"), "holder/root")
	if holder := lineOf(`  id: "holder"`); root.Line > holder || root.LastLine < holder || root.Across[4] != 560 || root.Across[5] != 220 {
		t.Errorf("a node of a template that is not written again points at lines %d to %d and stands at %v, %v; want the template node around line %d, and the same place", root.Line, root.LastLine, root.Across[4], root.Across[5], holder)
	}
}

const aGUIWithLayers = `textures {
  name: "ui"
  texture: "/main/ui.atlas"
}
textures {
  name: "logo"
  texture: "/main/wide.png"
}
fonts {
  name: "small"
  font: "/main/small.font"
}
nodes {
  size {
    x: 10.0
    y: 10.0
  }
  type: TYPE_BOX
  id: "front"
  layer: "top"
}
nodes {
  size {
    x: 40.0
    y: 20.0
  }
  color {
    x: 0.5
    z: 0.0
  }
  type: TYPE_BOX
  id: "back"
  inherit_alpha: true
  alpha: 0.5
}
nodes {
  type: TYPE_BOX
  texture: "ui/tall"
  id: "child_of_back"
  parent: "back"
  inherit_alpha: true
  slice9 {
    x: 2.0
    y: 3.0
    z: 4.0
    w: 5.0
  }
  alpha: 0.5
  size_mode: SIZE_MODE_AUTO
}
nodes {
  type: TYPE_BOX
  texture: "logo"
  id: "whole_image"
  parent: "back"
  layer: "below"
  size_mode: SIZE_MODE_AUTO
  enabled: false
}
nodes {
  size {
    x: 100.0
    y: 30.0
  }
  type: TYPE_TEXT
  text: "score"
  font: "small"
  id: "under_what_is_off"
  pivot: PIVOT_SE
  parent: "whole_image"
}
nodes {
  size {
    x: 30.0
    y: 30.0
  }
  type: TYPE_PIE
  id: "hidden"
  outerBounds: PIEBOUNDS_RECTANGLE
  innerRadius: 5.0
  pieFillAngle: 90.0
  visible: false
}
nodes {
  type: TYPE_PARTICLEFX
  id: "sparks"
  parent: "hidden"
}
layers {
  name: "below"
}
layers {
  name: "top"
}
`

// A GUI is drawn layer by layer, and inside a layer in the order of its tree.
// A node without a layer goes with its parent's, and one with none at all is
// drawn before every layer. What a node carries besides its place goes to the
// drawing as it is written: the colour, the alpha it gets from its parent when
// it asks for it, the edges that are kept when it is stretched, and whether
// the game would show it at all.
func TestAGUIIsDrawnInTheOrderOfItsLayers(t *testing.T) {
	project := studyGUI(t)
	project["main/hud.gui"] = &fstest.MapFile{Data: []byte(aGUIWithLayers)}
	project["main/small.font"] = &fstest.MapFile{Data: []byte("font: \"/main/small.ttf\"\nmaterial: \"/builtins/fonts/font.material\"\nsize: 18\n")}

	laidOut := sceneOf(t, project, "/main/hud.gui")

	var order []string
	for _, item := range laidOut.Items {
		order = append(order, item.Kind+" "+item.Owner+" in "+strconv.Quote(item.Layer))
	}
	want := []string{
		`display  in ""`, `panel back in ""`, `sprite child_of_back in ""`, `pie hidden in ""`, `marker sparks in ""`,
		`sprite whole_image in "below"`, `label under_what_is_off in "below"`, `panel front in "top"`,
	}
	if strings.Join(order, "\n") != strings.Join(want, "\n") {
		t.Fatalf("drawn in this order:\n%s\nwant:\n%s", strings.Join(order, "\n"), strings.Join(want, "\n"))
	}
	back, child := nodeOf(t, laidOut, "back"), nodeOf(t, laidOut, "child_of_back")
	if back.Tint != "#80ff00" || back.Alpha != 0.5 || child.Alpha != 0.25 || child.Tint != "#ffffff" {
		t.Errorf("back is %s at %v and its child %s at %v; want #80ff00 at 0.5 and #ffffff at 0.25", back.Tint, back.Alpha, child.Tint, child.Alpha)
	}
	if child.Width != 16 || child.Height != 40 || len(child.Slice9) != 4 || child.Slice9[0] != 2 || child.Slice9[3] != 5 || laidOut.Images[child.Image].Path != "/main/tall.png" {
		t.Errorf("the node with a texture is %+v", child)
	}
	whole := nodeOf(t, laidOut, "whole_image")
	if whole.Kind != KindOfASprite || whole.Width != 48 || whole.Height != 16 || laidOut.Images[whole.Image].Path != "/main/wide.png" || len(whole.Slice9) != 0 {
		t.Errorf("an image used as it is gave %+v", whole)
	}
	text := nodeOf(t, laidOut, "under_what_is_off")
	if !whole.IsOff || !text.IsOff || text.Text != "score" || text.TextSize != 18 || text.LeanX != 1 || text.LeanY != -1 {
		t.Errorf("what is under a node that is not enabled is %+v; want it off as well, 18 high, leaning right and down", text)
	}
	hidden, sparks := nodeOf(t, laidOut, "hidden"), nodeOf(t, laidOut, "sparks")
	if !hidden.IsOff || sparks.IsOff || hidden.Inner != 5 || hidden.Sweep != 90 || !hidden.IsSquare || sparks.Type != "particlefx" {
		t.Errorf("a node that is not visible is %+v and what it holds %+v; want only the node itself off", hidden, sparks)
	}
}

// The frame a GUI is laid out in is the display the game asks for, and Defold's
// own 960 by 640 when it asks for none. A change of game.project is a change of
// the drawing, so it is among what the scene uses.
func TestAGUIIsFramedByTheDisplayOfTheGame(t *testing.T) {
	project := studyGUI(t)
	project["main/hud.gui"] = &fstest.MapFile{Data: []byte(aGUIWithLayers)}

	asItComes := sceneOf(t, project, "/main/hud.gui")
	project["game.project"] = &fstest.MapFile{Data: []byte("[project]\ntitle = Study\n\n[display]\nwidth = 1280\nheight = 720\nhigh_dpi = 1\n")}
	asAsked := sceneOf(t, project, "/main/hud.gui")

	if asItComes.Display == nil || *asItComes.Display != [2]float64{960, 640} {
		t.Errorf("without a display the frame is %v", asItComes.Display)
	}
	frame := asAsked.Items[0]
	if asAsked.Display == nil || *asAsked.Display != [2]float64{1280, 720} || frame.Kind != KindOfTheDisplay || frame.Width != 1280 || frame.Across[4] != 640 || frame.Across[5] != 360 {
		t.Errorf("the frame is %v, drawn as %+v", asAsked.Display, frame)
	}
	if asAsked.MinX > -20 || asAsked.MaxX != 1280 || asAsked.MaxY != 720 || !strings.Contains(strings.Join(asAsked.Uses, " "), "/game.project") {
		t.Errorf("the scene reaches from %v to %v across and uses %v; want the frame and the nodes outside it, and game.project among what it uses", asAsked.MinX, asAsked.MaxX, asAsked.Uses)
	}
}

// A template may hold a template, and nothing stops two from holding each
// other, or a handful from holding one another ten times each. What is laid
// out has an end in both cases.
func TestAGUIOfTemplatesHasAnEnd(t *testing.T) {
	template := func(scene string) string {
		return "nodes {\n  type: TYPE_TEMPLATE\n  id: \"again\"\n  template: \"" + scene + "\"\n}\nnodes {\n  size {\n    x: 4.0\n    y: 4.0\n  }\n  type: TYPE_BOX\n  id: \"box\"\n}\n"
	}
	project := fstest.MapFS{
		"main/one.gui": {Data: []byte(template("/main/two.gui"))},
		"main/two.gui": {Data: []byte(template("/main/one.gui"))},
	}
	var many strings.Builder
	for index := range 10 {
		many.WriteString("nodes {\n  type: TYPE_TEMPLATE\n  id: \"t" + strconv.Itoa(index) + "\"\n  template: \"/main/level%d.gui\"\n}\n")
	}
	for level := range 8 {
		project["main/level"+strconv.Itoa(level)+".gui"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(many.String(), "%d", strconv.Itoa(level+1)))}
	}
	project["main/level8.gui"] = &fstest.MapFile{Data: []byte("nodes {\n  size {\n    x: 4.0\n    y: 4.0\n  }\n  type: TYPE_BOX\n  id: \"box\"\n}\n")}

	around := sceneOf(t, project, "/main/one.gui")
	if len(around.Items) != 3 || around.IsCutShort {
		t.Errorf("two scenes that hold each other gave %d items (cut short: %v); want the two boxes and the display", len(around.Items), around.IsCutShort)
	}
	exploding := sceneOf(t, project, "/main/level0.gui")
	if !exploding.IsCutShort || exploding.Objects > MostNodes || len(exploding.Items) > MostNodes+1 {
		t.Errorf("a scene of a hundred million nodes gave %d of them (cut short: %v); want it cut short at %d", exploding.Objects, exploding.IsCutShort, MostNodes)
	}
}
