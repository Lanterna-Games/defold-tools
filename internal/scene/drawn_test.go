package scene

import (
	"slices"
	"testing"
	"testing/fstest"
)

func drawnGame(t *testing.T) fstest.MapFS {
	t.Helper()
	project := studyProject(t)
	project["main/quarters.png"] = &fstest.MapFile{Data: pictureOf(t, 32, 32)}
	project["main/ramp.png"] = &fstest.MapFile{Data: pictureOf(t, 48, 48)}
	project["main/dot.png"] = &fstest.MapFile{Data: pictureOf(t, 4, 4)}
	return project
}

// The numbers are read off a picture the engine drew: testdata/README.md says
// how. An image is placed by its pivot, which is counted from the left and from
// the top of the image, on a sprite and on a box of a GUI alike, where it adds
// to the pivot of the node. An animation that is flipped stays where it is and
// is mirrored. The edges a sprite or a box keeps are left, top, right, bottom.
func TestImagesAreWhereTheEngineDrawsThem(t *testing.T) {
	cases := []struct {
		file      string
		owner     string
		component string
		left      float64
		right     float64
		bottom    float64
		top       float64
		flip      int
		kept      []float64
	}{
		{"/main/things.go", "", "plain", 34, 66, 234, 266, 0, nil},
		{"/main/things.go", "", "mirrored", 104, 136, 234, 266, FlippedAcross, nil},
		{"/main/things.go", "", "upside_down", 174, 206, 234, 266, FlippedUpDown, nil},
		{"/main/things.go", "", "pivoted", 272, 304, 250, 282, 0, nil},
		{"/main/things.go", "", "sliced", 40, 160, 100, 180, 0, []float64{4, 8, 12, 16}},
		{"/main/things.go", "", "tile_mirrored", 204, 236, 124, 156, FlippedAcross, nil},
		{"/main/hud.gui", "sliced", "", 240, 360, 100, 180, 0, []float64{4, 8, 12, 16}},
		{"/main/hud.gui", "mirrored_box", "", 304, 336, 34, 66, FlippedAcross, nil},
		{"/main/hud.gui", "pivoted_box", "", 352, 384, 250, 282, 0, nil},
		{"/main/hud.gui", "pivoted_corner", "", 158, 190, 36, 68, 0, nil},
	}
	project := drawnGame(t)
	scenes := map[string]*Scene{}
	for _, testCase := range cases {
		t.Run(testCase.file+" "+testCase.owner+testCase.component, func(t *testing.T) {
			if scenes[testCase.file] == nil {
				scenes[testCase.file] = sceneOf(t, project, testCase.file)
			}
			item := itemOf(t, scenes[testCase.file], testCase.owner, testCase.component, KindOfASprite)
			left, right := item.Across[4]-item.Width/2, item.Across[4]+item.Width/2
			bottom, top := item.Across[5]-item.Height/2, item.Across[5]+item.Height/2
			if left != testCase.left || right != testCase.right || bottom != testCase.bottom || top != testCase.top {
				t.Errorf("it covers %v to %v across and %v to %v up, the engine draws it from %v to %v and from %v to %v", left, right, bottom, top, testCase.left, testCase.right, testCase.bottom, testCase.top)
			}
			if item.Flip != testCase.flip || !slices.Equal(item.Slice9, testCase.kept) {
				t.Errorf("flip = %d and the edges kept are %v; want %d and %v", item.Flip, item.Slice9, testCase.flip, testCase.kept)
			}
		})
	}
}

// The engine draws a pie from the right, turning counterclockwise for a fill
// that is positive and clockwise for one that is negative, around the hole
// its inner radius leaves. The numbers of the text are passed on as they are.
func TestAPieIsPassedOnAsItIsWritten(t *testing.T) {
	laidOut := sceneOf(t, drawnGame(t), "/main/hud.gui")

	quarter, negative := nodeOf(t, laidOut, "quarter_pie"), nodeOf(t, laidOut, "negative_pie")

	if quarter.Kind != KindOfAPie || quarter.Sweep != 90 || quarter.Inner != 0 || quarter.Tint != "#ff8000" {
		t.Errorf("a quarter of a pie is %+v", quarter)
	}
	if negative.Sweep != -120 || negative.Inner != 20 || negative.Across[4] != 200 || negative.Across[5] != 50 {
		t.Errorf("a pie filled the other way is %+v", negative)
	}
}
