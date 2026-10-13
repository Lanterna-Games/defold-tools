package check

const syntaxCheck = "syntax"

func syntaxErrors(inspected inspection) []Problem {
	loaded := inspected.project
	problems := make([]Problem, 0, len(loaded.SyntaxErrors))
	for _, syntaxError := range loaded.SyntaxErrors {
		problems = append(problems, Problem{
			Check:    syntaxCheck,
			Location: syntaxError.Location,
			Message:  syntaxError.Message,
		})
	}
	return problems
}
