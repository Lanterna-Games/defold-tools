package textureset

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	CompiledSuffixOfAnAtlas = ".a.texturesetc"

	fieldOfTheWidth        = 2
	fieldOfTheHeight       = 3
	fieldOfAnAnimation     = 5
	fieldOfTheFrameIndices = 13
	fieldOfTheCoordinates  = 14
	fieldOfThePageIndices  = 18
	fieldOfThePageCount    = 19

	fieldOfTheID        = 1
	fieldOfTheStart     = 4
	fieldOfTheEnd       = 5
	wireOfAVarint       = 0
	wireOfEightBytes    = 1
	wireOfBytes         = 2
	wireOfFourBytes     = 5
	bitsOfTheWire       = 3
	maskOfTheWire       = 7
	bitsOfAVarintByte   = 7
	maskOfAVarintByte   = 0x7f
	markOfMoreToCome    = 0x80
	bytesOfAFloat       = 4
	floatsOfAQuad       = 8
	longestVarintInBits = 64
)

var ErrNotATextureSet = errors.New("this is not a texture set as Defold's build tool writes it")

type Frame struct {
	Page      int
	X         int
	Y         int
	Width     int
	Height    int
	IsRotated bool
}

type Animation struct {
	ID     string
	Frames []Frame
}

type Set struct {
	Width      int
	Height     int
	Pages      int
	Animations []Animation
}

type field struct {
	number int
	wire   int
	value  uint64
	bytes  []byte
}

func varintAt(encoded []byte) (value uint64, width int, isWhole bool) {
	for shift := 0; shift < longestVarintInBits; shift += bitsOfAVarintByte {
		if width >= len(encoded) {
			return 0, 0, false
		}
		read := encoded[width]
		width++
		value |= uint64(read&maskOfAVarintByte) << shift
		if read&markOfMoreToCome == 0 {
			return value, width, true
		}
	}
	return 0, 0, false
}

func fieldsOf(encoded []byte) ([]field, error) {
	var fields []field
	for len(encoded) > 0 {
		key, width, isWhole := varintAt(encoded)
		if !isWhole {
			return nil, ErrNotATextureSet
		}
		encoded = encoded[width:]
		read := field{number: int(key >> bitsOfTheWire), wire: int(key & maskOfTheWire)}
		taken := 0
		switch read.wire {
		case wireOfAVarint:
			read.value, taken, isWhole = varintAt(encoded)
		case wireOfEightBytes:
			taken, isWhole = binary.Size(uint64(0)), len(encoded) >= binary.Size(uint64(0))
		case wireOfFourBytes:
			taken, isWhole = bytesOfAFloat, len(encoded) >= bytesOfAFloat
		case wireOfBytes:
			var length uint64
			length, taken, isWhole = varintAt(encoded)
			left := uint64(max(len(encoded)-taken, 0))
			if isWhole && length <= left {
				read.bytes = encoded[taken : taken+int(length)] //nolint:gosec // checked against what is left just above
				taken += int(length)                            //nolint:gosec // as above
			} else {
				isWhole = false
			}
		default:
			isWhole = false
		}
		if !isWhole {
			return nil, ErrNotATextureSet
		}
		encoded = encoded[taken:]
		fields = append(fields, read)
	}
	return fields, nil
}

func numbersOf(fields []field, number int) []int {
	var numbers []int
	for _, each := range fields {
		if each.number != number {
			continue
		}
		if each.wire == wireOfAVarint {
			numbers = append(numbers, int(each.value)) //nolint:gosec // an index or a count of a texture set
			continue
		}
		for packed := each.bytes; len(packed) > 0; {
			value, width, isWhole := varintAt(packed)
			if !isWhole {
				break
			}
			numbers = append(numbers, int(value)) //nolint:gosec // as above
			packed = packed[width:]
		}
	}
	return numbers
}

func numberOf(fields []field, number int) int {
	if numbers := numbersOf(fields, number); len(numbers) > 0 {
		return numbers[0]
	}
	return 0
}

func bytesOf(fields []field, number int) []byte {
	for _, each := range fields {
		if each.number == number && each.wire == wireOfBytes {
			return each.bytes
		}
	}
	return nil
}

func frameAt(coordinates []byte, quad, width, height int) (Frame, bool) {
	start := quad * floatsOfAQuad * bytesOfAFloat
	if quad < 0 || start+floatsOfAQuad*bytesOfAFloat > len(coordinates) {
		return Frame{}, false
	}
	var across, upwards [floatsOfAQuad / 2]float64
	for corner := range across {
		at := start + corner*2*bytesOfAFloat
		across[corner] = float64(math.Float32frombits(binary.LittleEndian.Uint32(coordinates[at:])))
		upwards[corner] = float64(math.Float32frombits(binary.LittleEndian.Uint32(coordinates[at+bytesOfAFloat:])))
	}
	left, right := min(across[0], across[1], across[2], across[3]), max(across[0], across[1], across[2], across[3])
	bottom, top := min(upwards[0], upwards[1], upwards[2], upwards[3]), max(upwards[0], upwards[1], upwards[2], upwards[3])
	return Frame{
		X:         int(math.Round(left * float64(width))),
		Y:         int(math.Round((1 - top) * float64(height))),
		Width:     int(math.Round((right - left) * float64(width))),
		Height:    int(math.Round((top - bottom) * float64(height))),
		IsRotated: upwards[0] == upwards[1],
	}, true
}

func Read(compiled []byte) (*Set, error) {
	fields, err := fieldsOf(compiled)
	if err != nil {
		return nil, err
	}
	set := &Set{Width: numberOf(fields, fieldOfTheWidth), Height: numberOf(fields, fieldOfTheHeight), Pages: max(numberOf(fields, fieldOfThePageCount), 1)}
	coordinates := bytesOf(fields, fieldOfTheCoordinates)
	if set.Width <= 0 || set.Height <= 0 || coordinates == nil {
		return nil, ErrNotATextureSet
	}
	quadOf, pageOf := numbersOf(fields, fieldOfTheFrameIndices), numbersOf(fields, fieldOfThePageIndices)
	for _, each := range fields {
		if each.number != fieldOfAnAnimation || each.wire != wireOfBytes {
			continue
		}
		described, err := fieldsOf(each.bytes)
		if err != nil {
			return nil, err
		}
		animation := Animation{ID: string(bytesOf(described, fieldOfTheID))}
		for index := numberOf(described, fieldOfTheStart); index < numberOf(described, fieldOfTheEnd); index++ {
			if index >= len(quadOf) {
				return nil, fmt.Errorf("%w: animation %q runs past its frames", ErrNotATextureSet, animation.ID)
			}
			frame, isThere := frameAt(coordinates, quadOf[index], set.Width, set.Height)
			if !isThere {
				return nil, fmt.Errorf("%w: animation %q has a frame without a place", ErrNotATextureSet, animation.ID)
			}
			if quadOf[index] < len(pageOf) {
				frame.Page = pageOf[quadOf[index]]
			}
			animation.Frames = append(animation.Frames, frame)
		}
		set.Animations = append(set.Animations, animation)
	}
	return set, nil
}

func (s *Set) Animation(id string) (animation Animation, isThere bool) {
	for _, each := range s.Animations {
		if each.ID == id {
			return each, true
		}
	}
	return Animation{}, false
}
