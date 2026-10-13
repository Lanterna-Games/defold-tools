package textproto

import (
	"strconv"

	"github.com/Lanterna-Games/defold-tools/internal/source"
)

type Scalar struct {
	Text     string
	Quoted   bool
	Position source.Position

	segments []segment
	locate   source.Locator
}

type segment struct {
	rawStart     int
	raw          string
	decodedStart int
}

func (s *Scalar) PositionAt(decodedOffset int) source.Position {
	if len(s.segments) == 0 {
		return s.locate(s.Position.Offset + decodedOffset)
	}
	chosen := s.segments[0]
	for _, candidate := range s.segments {
		if candidate.decodedStart > decodedOffset {
			break
		}
		chosen = candidate
	}
	return s.locate(chosen.rawStart + rawOffsetOf(chosen.raw, decodedOffset-chosen.decodedStart))
}

func (s *Scalar) TextPosition() source.Position {
	return s.PositionAt(0)
}

func (s *Scalar) Embedded() (*Message, []SyntaxError) {
	return Parse([]byte(s.Text), s.PositionAt)
}

func rawOffsetOf(raw string, decodedOffset int) int {
	rawOffset, decoded := 0, 0
	for rawOffset < len(raw) && decoded < decodedOffset {
		bytes, width := decodeAt(raw, rawOffset)
		decoded += len(bytes)
		rawOffset += width
	}
	return rawOffset
}

func decode(raw string) string {
	decoded := make([]byte, 0, len(raw))
	for offset := 0; offset < len(raw); {
		bytes, width := decodeAt(raw, offset)
		decoded = append(decoded, bytes...)
		offset += width
	}
	return string(decoded)
}

var simpleEscapes = map[byte]byte{
	'n': '\n', 't': '\t', 'r': '\r', 'a': '\a', 'b': '\b', 'f': '\f', 'v': '\v',
	'\\': '\\', '"': '"', '\'': '\'', '?': '?',
}

func decodeAt(raw string, offset int) (decoded []byte, width int) {
	if raw[offset] != '\\' || offset+1 >= len(raw) {
		return []byte{raw[offset]}, 1
	}
	escaped := raw[offset+1]
	if replacement, isSimple := simpleEscapes[escaped]; isSimple {
		return []byte{replacement}, 2
	}
	if escaped == 'x' || escaped == 'X' {
		return decodeNumeric(raw, offset, 2, 2, 16)
	}
	if escaped >= '0' && escaped <= '7' {
		return decodeNumeric(raw, offset, 1, 3, 8)
	}
	return []byte{escaped}, 2
}

func decodeNumeric(raw string, offset, prefixWidth, maxDigits, base int) (decoded []byte, width int) {
	start := offset + prefixWidth
	end := start
	for end < len(raw) && end-start < maxDigits && isDigitOfBase(raw[end], base) {
		end++
	}
	value, err := strconv.ParseUint(raw[start:end], base, 8)
	if err != nil {
		return []byte{raw[offset+1]}, 2
	}
	return []byte{byte(value)}, end - offset
}

func isDigitOfBase(character byte, base int) bool {
	switch {
	case character >= '0' && character <= '7':
		return true
	case base == 16:
		return (character >= '8' && character <= '9') ||
			(character >= 'a' && character <= 'f') ||
			(character >= 'A' && character <= 'F')
	default:
		return false
	}
}
