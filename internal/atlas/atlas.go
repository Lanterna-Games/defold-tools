package atlas

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
)

const (
	Extension       = ".atlas"
	startOfABlock   = "images {"
	lineBreakOfUnix = "\n"
	lineBreakOfDOS  = "\r\n"
)

var (
	ExtensionsOfImages = []string{".png", ".jpg", ".jpeg"}
	imageOfABlock      = regexp.MustCompile(`(?m)^\s*image:\s*"([^"]*)"`)
)

type block struct {
	start int
	end   int
	image string
}

func blocksOf(content []byte) []block {
	var found []block
	depth, inAString, lineStart, opened := 0, false, 0, -1
	for offset := 0; offset < len(content); offset++ {
		character := content[offset]
		switch {
		case inAString && character == '\\':
			offset++
		case character == '"':
			inAString = !inAString
		case inAString:
		case character == '\n':
			lineStart = offset + 1
		case character == '{':
			if depth == 0 && strings.HasPrefix(string(content[lineStart:offset+1]), startOfABlock) {
				opened = lineStart
			}
			depth++
		case character == '}':
			depth--
			if depth == 0 && opened >= 0 {
				end := offset + 1
				for end < len(content) && (content[end] == '\r' || content[end] == '\n') {
					end++
					if content[end-1] == '\n' {
						break
					}
				}
				image := ""
				if named := imageOfABlock.FindSubmatch(content[opened:end]); named != nil {
					image = string(named[1])
				}
				found = append(found, block{start: opened, end: end, image: image})
				opened = -1
			}
		}
	}
	return found
}

func ImagesOf(content []byte) []string {
	blocks := blocksOf(content)
	images := make([]string, 0, len(blocks))
	for _, each := range blocks {
		images = append(images, each.image)
	}
	return images
}

func lineBreakOf(content []byte) string {
	if bytes.Contains(content, []byte(lineBreakOfDOS)) {
		return lineBreakOfDOS
	}
	return lineBreakOfUnix
}

func With(content []byte, image string) []byte {
	lineBreak := lineBreakOf(content)
	added := startOfABlock + lineBreak + `  image: "` + image + `"` + lineBreak + "}" + lineBreak
	at := 0
	if blocks := blocksOf(content); len(blocks) > 0 {
		at = blocks[len(blocks)-1].end
		if endsWithoutALineBreak := at == len(content) && !bytes.HasSuffix(content, []byte("\n")); endsWithoutALineBreak {
			added = lineBreak + strings.TrimSuffix(added, lineBreak)
		}
	}
	return slices.Concat(content[:at], []byte(added), content[at:])
}

func Without(content []byte, image string) []byte {
	kept := slices.Clone(content)
	for _, each := range slices.Backward(blocksOf(content)) {
		if each.image == image {
			kept = slices.Concat(kept[:each.start], kept[each.end:])
		}
	}
	return kept
}

type Change struct {
	Added   []string
	Removed []string
}

func Synced(content []byte, there []string, isStillThere func(image string) bool) (synced []byte, change Change) {
	synced = content
	listed := ImagesOf(content)
	for _, image := range listed {
		if !isStillThere(image) && !slices.Contains(change.Removed, image) {
			synced = Without(synced, image)
			change.Removed = append(change.Removed, image)
		}
	}
	isAFrame := map[string]bool{}
	for _, entry := range EntriesOf(content) {
		isAFrame[entry.Image] = entry.Animation != "" || isAFrame[entry.Image]
	}
	for _, image := range there {
		if !slices.Contains(listed, image) && !slices.Contains(change.Added, image) && !isAFrame[image] {
			synced = With(synced, image)
			change.Added = append(change.Added, image)
		}
	}
	return synced, change
}
