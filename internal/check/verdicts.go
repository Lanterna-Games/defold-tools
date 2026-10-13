package check

import "github.com/Lanterna-Games/defold-tools/internal/project"

type rejection struct {
	check   string
	message string
}

type verdicts struct {
	check       string
	timesJudged map[project.Location]int
	rejections  map[project.Location][]rejection
}

func newVerdicts(check string) verdicts {
	return verdicts{check: check, timesJudged: map[project.Location]int{}, rejections: map[project.Location][]rejection{}}
}

func (v verdicts) accept(location project.Location) {
	v.timesJudged[location]++
}

func (v verdicts) reject(location project.Location, message string) {
	v.rejectAs(v.check, location, message)
}

func (v verdicts) rejectAs(check string, location project.Location, message string) {
	v.timesJudged[location]++
	v.rejections[location] = append(v.rejections[location], rejection{check: check, message: message})
}

func (v verdicts) problems() []Problem {
	var problems []Problem
	for location, rejections := range v.rejections {
		isRejectedWhereverItIsJudged := len(rejections) == v.timesJudged[location]
		if isRejectedWhereverItIsJudged {
			problems = append(problems, Problem{Check: rejections[0].check, Location: location, Message: rejections[0].message})
		}
	}
	return problems
}
