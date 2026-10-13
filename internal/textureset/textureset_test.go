package textureset

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"slices"
	"testing"
)

func setOfTheStudyAtlas(t *testing.T) *Set {
	t.Helper()
	compiled, err := os.ReadFile("testdata/props.a.texturesetc")
	if err != nil {
		t.Fatal(err)
	}
	set, err := Read(compiled)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// An atlas has no coordinates: Defold packs the images when it builds, and
// writes where each one landed in the texture set. The places below were read
// off the texture of that same build, where each image was found by a mark at
// its top left corner, so they are where the pixels are and not only what this
// reader makes of the numbers. They are measured from the top left of the page,
// which is where a picture is drawn from, while the file counts upwards.
func TestWhereTheBuildPlacedEachImage(t *testing.T) {
	set := setOfTheStudyAtlas(t)

	if set.Width != 128 || set.Height != 128 || set.Pages != 1 {
		t.Errorf("page = %dx%d, %d of them; want one of 128x128", set.Width, set.Height, set.Pages)
	}
	want := map[string][]Frame{
		"walk":  {{X: 96, Y: 40, Width: 24, Height: 24}, {X: 96, Y: 70, Width: 24, Height: 24}},
		"coin":  {{X: 96, Y: 100, Width: 16, Height: 16}},
		"house": {{X: 4, Y: 40, Width: 48, Height: 64, IsRotated: true}},
		"sign":  {{X: 4, Y: 4, Width: 100, Height: 30}},
		"tree":  {{X: 58, Y: 40, Width: 32, Height: 80}},
	}
	if len(set.Animations) != len(want) {
		t.Errorf("animations = %+v, want %d of them", set.Animations, len(want))
	}
	for id, frames := range want {
		animation, isThere := set.Animation(id)
		if !isThere || !slices.Equal(animation.Frames, frames) {
			t.Errorf("%s = %+v, want %+v", id, animation.Frames, frames)
		}
	}
	if _, isThere := set.Animation("walk_1"); isThere {
		t.Error("an image of an animation was found as an animation of its own")
	}
}

func encodedVarint(value uint64) []byte {
	var encoded []byte
	for value >= markOfMoreToCome {
		encoded = append(encoded, byte(value)|markOfMoreToCome)
		value >>= bitsOfAVarintByte
	}
	return append(encoded, byte(value))
}

func encodedNumber(number int, value uint64) []byte {
	return append(encodedVarint(uint64(number)<<bitsOfTheWire|wireOfAVarint), encodedVarint(value)...)
}

func encodedBytes(number int, content []byte) []byte {
	encoded := append(encodedVarint(uint64(number)<<bitsOfTheWire|wireOfBytes), encodedVarint(uint64(len(content)))...)
	return append(encoded, content...)
}

func encodedQuad(corners ...float32) []byte {
	var encoded []byte
	for _, corner := range corners {
		encoded = binary.LittleEndian.AppendUint32(encoded, math.Float32bits(corner))
	}
	return encoded
}

func encodedAnimation(id string, start, end uint64) []byte {
	return slices.Concat(encodedBytes(fieldOfTheID, []byte(id)), encodedNumber(fieldOfTheStart, start), encodedNumber(fieldOfTheEnd, end))
}

// An atlas too big for one page is split over several of the same size, and
// each image says which page it is on. Numbers that repeat may be written one
// by one, as here, or packed into one field, as the build tool writes them.
func TestAnImageSaysWhichPageItIsOn(t *testing.T) {
	compiled := slices.Concat(
		encodedNumber(fieldOfTheWidth, 64), encodedNumber(fieldOfTheHeight, 32),
		encodedBytes(fieldOfAnAnimation, encodedAnimation("ball", 0, 1)),
		encodedBytes(fieldOfAnAnimation, encodedAnimation("wall", 1, 2)),
		encodedNumber(fieldOfTheFrameIndices, 0), encodedNumber(fieldOfTheFrameIndices, 1),
		encodedBytes(fieldOfTheCoordinates, slices.Concat(
			encodedQuad(0, 1, 0, 0.5, 0.25, 0.5, 0.25, 1),
			encodedQuad(0.5, 0.5, 0.5, 0, 1, 0, 1, 0.5),
		)),
		encodedNumber(fieldOfThePageIndices, 0), encodedNumber(fieldOfThePageIndices, 1),
		encodedNumber(fieldOfThePageCount, 2),
	)

	set, err := Read(compiled)
	if err != nil {
		t.Fatal(err)
	}

	ball, _ := set.Animation("ball")
	wall, _ := set.Animation("wall")
	if set.Pages != 2 || !slices.Equal(ball.Frames, []Frame{{Page: 0, X: 0, Y: 0, Width: 16, Height: 16}}) || !slices.Equal(wall.Frames, []Frame{{Page: 1, X: 32, Y: 16, Width: 32, Height: 16}}) {
		t.Errorf("pages = %d, ball = %+v, wall = %+v", set.Pages, ball.Frames, wall.Frames)
	}
}

// What is read comes from a build that may have been cut short, or from a file
// that is something else. Neither may be taken for a layout.
func TestWhatIsNotATextureSetIsRefused(t *testing.T) {
	whole, err := os.ReadFile("testdata/props.a.texturesetc")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"nothing":                         {},
		"cut in the middle":               whole[:len(whole)/2],
		"text":                            []byte("images {\n  image: \"/main/ball.png\"\n}\n"),
		"a size and no coordinates":       slices.Concat(encodedNumber(fieldOfTheWidth, 64), encodedNumber(fieldOfTheHeight, 64)),
		"an animation past its frames":    slices.Concat(encodedNumber(fieldOfTheWidth, 64), encodedNumber(fieldOfTheHeight, 64), encodedBytes(fieldOfTheCoordinates, encodedQuad(0, 1, 0, 0, 1, 0, 1, 1)), encodedBytes(fieldOfAnAnimation, encodedAnimation("ball", 0, 3)), encodedNumber(fieldOfTheFrameIndices, 0)),
		"a frame without its coordinates": slices.Concat(encodedNumber(fieldOfTheWidth, 64), encodedNumber(fieldOfTheHeight, 64), encodedBytes(fieldOfTheCoordinates, encodedQuad(0, 1, 0, 0, 1, 0, 1, 1)), encodedBytes(fieldOfAnAnimation, encodedAnimation("ball", 0, 1)), encodedNumber(fieldOfTheFrameIndices, 4)),
	}
	for name, compiled := range cases {
		t.Run(name, func(t *testing.T) {
			if set, err := Read(compiled); !errors.Is(err, ErrNotATextureSet) {
				t.Errorf("read %+v, %v; want it refused", set, err)
			}
		})
	}
}
