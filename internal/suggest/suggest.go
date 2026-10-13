package suggest

func Closest(name string, candidates []string) (closest string, found bool) {
	bestDistance := maxDistanceWorthSuggesting(name) + 1
	for _, candidate := range candidates {
		if distance := typingDistance(name, candidate); distance < bestDistance {
			closest, bestDistance, found = candidate, distance, true
		}
	}
	return closest, found
}

func maxDistanceWorthSuggesting(name string) int {
	return max(1, len(name)/3)
}

func typingDistance(typed, intended string) int {
	rows := make([][]int, len(typed)+1)
	for row := range rows {
		rows[row] = make([]int, len(intended)+1)
		rows[row][0] = row
	}
	for column := range rows[0] {
		rows[0][column] = column
	}
	for row := 1; row <= len(typed); row++ {
		for column := 1; column <= len(intended); column++ {
			substitution := rows[row-1][column-1]
			if typed[row-1] != intended[column-1] {
				substitution++
			}
			distance := min(substitution, rows[row-1][column]+1, rows[row][column-1]+1)
			if isSwapOfNeighbours(typed, intended, row, column) {
				distance = min(distance, rows[row-2][column-2]+1)
			}
			rows[row][column] = distance
		}
	}
	return rows[len(typed)][len(intended)]
}

func isSwapOfNeighbours(typed, intended string, row, column int) bool {
	return row > 1 && column > 1 &&
		typed[row-1] == intended[column-2] &&
		typed[row-2] == intended[column-1]
}
