package scene

import (
	"math"
	"strconv"

	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

type Transform [3][4]float64

func identity() Transform {
	return Transform{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}
}

func (t Transform) Times(inner Transform) Transform {
	var product Transform
	for row := range 3 {
		for column := range 4 {
			for step := range 3 {
				product[row][column] += t[row][step] * inner[step][column]
			}
		}
		product[row][3] += t[row][3]
	}
	return product
}

func (t Transform) Across() [6]float64 {
	return [6]float64{t[0][0], t[1][0], t[0][1], t[1][1], t[0][3], t[1][3]}
}

func (t Transform) Depth() float64 {
	return t[2][3]
}

func (t Transform) At(x, y float64) (acrossed, raised float64) {
	return t[0][0]*x + t[0][1]*y + t[0][3], t[1][0]*x + t[1][1]*y + t[1][3]
}

func numberOf(message *textproto.Message, name string, otherwise float64) float64 {
	written, isWritten := message.Scalar(name)
	if !isWritten {
		return otherwise
	}
	number, err := strconv.ParseFloat(written.Text, 64)
	if err != nil {
		return otherwise
	}
	return number
}

func partOf(message *textproto.Message, name string) (part *textproto.Message, isWritten bool) {
	parts := message.Messages(name)
	if len(parts) == 0 {
		return nil, false
	}
	return parts[0], true
}

func placedBy(message *textproto.Message) Transform {
	placed := identity()
	if position, isWritten := partOf(message, "position"); isWritten {
		placed[0][3], placed[1][3], placed[2][3] = numberOf(position, "x", 0), numberOf(position, "y", 0), numberOf(position, "z", 0)
	}
	if rotation, isWritten := partOf(message, "rotation"); isWritten {
		x, y, z, w := numberOf(rotation, "x", 0), numberOf(rotation, "y", 0), numberOf(rotation, "z", 0), numberOf(rotation, "w", 1)
		if length := math.Sqrt(x*x + y*y + z*z + w*w); length > 0 {
			x, y, z, w = x/length, y/length, z/length, w/length
		}
		placed[0][0], placed[0][1], placed[0][2] = 1-2*(y*y+z*z), 2*(x*y-z*w), 2*(x*z+y*w)
		placed[1][0], placed[1][1], placed[1][2] = 2*(x*y+z*w), 1-2*(x*x+z*z), 2*(y*z-x*w)
		placed[2][0], placed[2][1], placed[2][2] = 2*(x*z-y*w), 2*(y*z+x*w), 1-2*(x*x+y*y)
	}
	scale := [3]float64{1, 1, 1}
	for _, name := range []string{"scale3", "scale"} {
		if written, isWritten := partOf(message, name); isWritten {
			scale = [3]float64{numberOf(written, "x", 1), numberOf(written, "y", 1), numberOf(written, "z", 1)}
			break
		}
		if written, isWritten := message.Scalar(name); isWritten {
			if uniform, err := strconv.ParseFloat(written.Text, 64); err == nil {
				scale = [3]float64{uniform, uniform, uniform}
				break
			}
		}
	}
	for row := range 3 {
		for column := range 3 {
			placed[row][column] *= scale[column]
		}
	}
	return placed
}
