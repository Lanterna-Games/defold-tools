package lsp

import (
	"bytes"
	"net/url"
	"path"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	schemeOfFiles       = "file"
	charactersOfAName   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_."
	charactersOfAString = "\"'"
)

func pathOf(uri string) (slashPath string, isAFile bool) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != schemeOfFiles {
		return "", false
	}
	slashPath = parsed.Path
	if hasADriveLetter(slashPath) {
		slashPath = strings.ToUpper(slashPath[1:2]) + slashPath[2:]
	}
	return slashPath, slashPath != ""
}

func hasADriveLetter(slashPath string) bool {
	return len(slashPath) >= 3 && slashPath[0] == '/' && slashPath[2] == ':'
}

func uriOf(slashPath string) string {
	if len(slashPath) >= 2 && slashPath[1] == ':' {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: schemeOfFiles, Path: slashPath}).String()
}

func relativeTo(root, slashPath string) (relative string, isInside bool) {
	root = strings.TrimSuffix(root, "/")
	if strings.EqualFold(root, slashPath) || !hasThePrefixOnThisSystem(slashPath, root+"/") {
		return "", false
	}
	return path.Clean(slashPath[len(root)+1:]), true
}

func hasThePrefixOnThisSystem(slashPath, prefix string) bool {
	if hasADriveLetter("/" + prefix) {
		return len(slashPath) >= len(prefix) && strings.EqualFold(slashPath[:len(prefix)], prefix)
	}
	return strings.HasPrefix(slashPath, prefix)
}

func rangeAt(content []byte, line, byteColumn int) textRange {
	text := lineOf(content, line)
	startByte := min(max(byteColumn-1, 0), len(text))
	endByte := startByte + lengthOfWhatStartsAt(text[startByte:])
	return textRange{
		Start: position{Line: line - 1, Character: utf16Length(text[:startByte])},
		End:   position{Line: line - 1, Character: utf16Length(text[:endByte])},
	}
}

func rangeOfAProblem(content []byte, line, byteColumn int) textRange {
	shown := rangeAt(content, line, byteColumn)
	text := lineOf(content, line)
	startByte := min(max(byteColumn-1, 0), len(text))
	rest := text[startByte:]
	name := lengthOfWhatStartsAt(rest)
	if name == 0 || name+1 >= len(rest) || rest[name] != '(' || strings.IndexByte(charactersOfAString, rest[name+1]) < 0 {
		return shown
	}
	closing := name + 1 + lengthOfWhatStartsAt(rest[name+1:])
	if closing >= len(rest) || rest[closing] != ')' {
		return shown
	}
	shown.End.Character = utf16Length(text[:startByte+closing+1])
	return shown
}

func startOfLine(content []byte, line int) int {
	offset := 0
	for current := 1; current < line; current++ {
		next := bytes.IndexByte(content[offset:], '\n')
		if next < 0 {
			return len(content)
		}
		offset += next + 1
	}
	return offset
}

func boundsAt(content []byte, line, byteColumn int) (start, end int) {
	text := lineOf(content, line)
	startByte := min(max(byteColumn-1, 0), len(text))
	lineStart := startOfLine(content, line)
	return lineStart + startByte, lineStart + startByte + lengthOfWhatStartsAt(text[startByte:])
}

func offsetAt(content []byte, where position) int {
	text := lineOf(content, where.Line+1)
	units, inBytes := 0, 0
	for _, character := range text {
		if units >= where.Character {
			break
		}
		units += len(utf16.Encode([]rune{character}))
		inBytes += utf8.RuneLen(character)
	}
	return min(startOfLine(content, where.Line+1)+inBytes, len(content))
}

func positionAt(content []byte, offset int) position {
	offset = min(max(offset, 0), len(content))
	line := bytes.Count(content[:offset], []byte("\n"))
	lineStart := bytes.LastIndexByte(content[:offset], '\n') + 1
	return position{Line: line, Character: utf16Length(string(content[lineStart:offset]))}
}

func lineOf(content []byte, line int) string {
	for current := 1; current < line; current++ {
		next := bytes.IndexByte(content, '\n')
		if next < 0 {
			return ""
		}
		content = content[next+1:]
	}
	text, _, _ := strings.Cut(string(content), "\n")
	return strings.TrimSuffix(text, "\r")
}

func lengthOfWhatStartsAt(text string) int {
	if text == "" {
		return 0
	}
	if quote := text[0]; strings.IndexByte(charactersOfAString, quote) >= 0 {
		if closing := strings.IndexByte(text[1:], quote); closing >= 0 {
			return closing + 2
		}
		return len(text)
	}
	length := len(text) - len(strings.TrimLeft(text, charactersOfAName))
	if length == 0 {
		_, length = utf8.DecodeRuneInString(text)
	}
	return length
}

func utf16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}
