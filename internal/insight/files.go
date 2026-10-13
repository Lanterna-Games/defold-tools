package insight

const charactersThatEndAPath = "\"\\\r\n"

func isOfAPath(character byte) bool {
	for index := range len(charactersThatEndAPath) {
		if charactersThatEndAPath[index] == character {
			return false
		}
	}
	return true
}

func ValueAround(content []byte, offset int) (start, end int) {
	start, end = offset, offset
	for start > 0 && isOfAPath(content[start-1]) {
		start--
	}
	for end < len(content) && isOfAPath(content[end]) {
		end++
	}
	return start, end
}

func (i Insight) FileNamedAt(content []byte, offset int) (resourcePath string, isAFile bool) {
	if offset < 0 || offset > len(content) {
		return "", false
	}
	start, end := ValueAround(content, offset)
	named := string(content[start:end])
	if len(named) < 2 || named[0] != '/' || !i.Project.HasFile(named) {
		return "", false
	}
	return named, true
}
