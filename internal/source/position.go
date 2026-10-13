package source

import "sort"

type Position struct {
	Offset int
	Line   int
	Column int
}

type Locator func(offset int) Position

func NewLocator(source []byte) Locator {
	lineStarts := []int{0}
	for offset, character := range source {
		if character == '\n' {
			lineStarts = append(lineStarts, offset+1)
		}
	}
	return func(offset int) Position {
		lineIndex := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > offset }) - 1
		return Position{Offset: offset, Line: lineIndex + 1, Column: offset - lineStarts[lineIndex] + 1}
	}
}
