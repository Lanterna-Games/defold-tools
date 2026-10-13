package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	side              = 256
	samplesPerSide    = 4
	unitsAcross       = 3.5
	halfArmWidth      = 0.5
	armLength         = 1.5
	halfHandleWidth   = 0.13
	headsApart        = 1.0
	radiusOfTheJaw    = 0.34
	halfJawOpening    = 0.14
	radiusOfTheRing   = 0.30
	radiusOfTheHole   = 0.13
	opaque            = 255
	permissionsOfIcon = 0o644
)

var (
	whereTheFaceStarts = color.RGBA{R: 0xFF, G: 0x3C, B: 0x2A, A: opaque}
	whereTheFaceEnds   = color.RGBA{R: 0xFF, G: 0xD2, B: 0x15, A: opaque}
)

func isInThePlus(x, y float64) bool {
	isInTheUprightBar := math.Abs(x) <= halfArmWidth && math.Abs(y) <= armLength
	isInTheLevelBar := math.Abs(y) <= halfArmWidth && math.Abs(x) <= armLength
	return isInTheUprightBar || isInTheLevelBar
}

func isInTheWrench(x, y float64) bool {
	isInTheHandle := math.Abs(x) <= halfHandleWidth && math.Abs(y) <= headsApart
	isInTheOpenEnd := math.Hypot(x, y-headsApart) <= radiusOfTheJaw && (math.Abs(x) > halfJawOpening || y < headsApart)
	fromTheRing := math.Hypot(x, y+headsApart)
	isInTheHole := fromTheRing <= radiusOfTheHole
	return (isInTheHandle || isInTheOpenEnd || fromTheRing <= radiusOfTheRing) && !isInTheHole
}

func mixed(from, to uint8, share float64) float64 {
	return float64(from) + (float64(to)-float64(from))*share
}

func colourAt(x, y float64) (red, green, blue float64) {
	share := math.Min(math.Max((x+y)/(2*armLength)+0.5, 0), 1)
	return mixed(whereTheFaceStarts.R, whereTheFaceEnds.R, share), mixed(whereTheFaceStarts.G, whereTheFaceEnds.G, share), mixed(whereTheFaceStarts.B, whereTheFaceEnds.B, share)
}

func drawn() *image.NRGBA {
	icon := image.NewNRGBA(image.Rect(0, 0, side, side))
	samples := float64(samplesPerSide * samplesPerSide)
	for row := range side {
		for column := range side {
			var red, green, blue, covered float64
			for sample := range samplesPerSide * samplesPerSide {
				acrossThePixel := (float64(sample%samplesPerSide) + 0.5) / samplesPerSide
				downThePixel := (float64(sample/samplesPerSide) + 0.5) / samplesPerSide
				x := ((float64(column)+acrossThePixel)/side - 0.5) * unitsAcross
				y := (0.5 - (float64(row)+downThePixel)/side) * unitsAcross
				if isInThePlus(x, y) && !isInTheWrench(x, y) {
					r, g, b := colourAt(x, y)
					red, green, blue, covered = red+r, green+g, blue+b, covered+1
				}
			}
			if covered > 0 {
				icon.SetNRGBA(column, row, color.NRGBA{
					R: uint8(math.Round(red / covered)), G: uint8(math.Round(green / covered)), B: uint8(math.Round(blue / covered)),
					A: uint8(math.Round(opaque * covered / samples)),
				})
			}
		}
	}
	return icon
}

func main() {
	output := flag.String("output", "editors/vscode/icon.png", "the image to write")
	flag.Parse()

	file, err := os.OpenFile(*output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissionsOfIcon)
	if err == nil {
		err = png.Encode(file, drawn())
		if closing := file.Close(); err == nil {
			err = closing
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "icon:", err)
		os.Exit(1)
	}
}
