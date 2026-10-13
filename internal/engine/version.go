package engine

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
)

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

type Version struct {
	Major int
	Minor int
	Patch int
}

var OldestSupported = Version{Major: 1, Minor: 6, Patch: 0}

func ParseVersion(text string) (version Version, isAVersion bool) {
	parts := versionPattern.FindStringSubmatch(text)
	if parts == nil {
		return Version{}, false
	}
	numbers := [3]int{}
	for index, part := range parts[1:] {
		number, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, false
		}
		numbers[index] = number
	}
	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2]}, true
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) Compare(other Version) int {
	return cmp.Or(cmp.Compare(v.Major, other.Major), cmp.Compare(v.Minor, other.Minor), cmp.Compare(v.Patch, other.Patch))
}

func (v Version) IsBefore(other Version) bool {
	return v.Compare(other) < 0
}
