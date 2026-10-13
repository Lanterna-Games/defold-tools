package lua

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func parse(t *testing.T, content string) *Chunk {
	t.Helper()
	chunk, errors := Parse([]byte(content))
	for _, err := range errors {
		t.Errorf("unexpected syntax error at %d:%d: %s", err.Position.Line, err.Position.Column, err.Message)
	}
	return chunk
}

// Renders an expression with every operation in parentheses, so that a test
// can state how it was grouped.
func grouped(expression Expression) string {
	switch typed := expression.(type) {
	case *NilLiteral:
		return "nil"
	case *VarargLiteral:
		return "..."
	case *BooleanLiteral:
		return strconv.FormatBool(typed.Value)
	case *NumberLiteral:
		return typed.Text
	case *StringLiteral:
		return fmt.Sprintf("%q", typed.Value)
	case *Identifier:
		return typed.Name
	case *FunctionLiteral:
		return "function"
	case *IndexExpression:
		return grouped(typed.Object) + "[" + grouped(typed.Index) + "]"
	case *CallExpression:
		arguments := make([]string, 0, len(typed.Arguments))
		for _, argument := range typed.Arguments {
			arguments = append(arguments, grouped(argument))
		}
		method := ""
		if typed.Method != nil {
			method = ":" + typed.Method.Name
		}
		return grouped(typed.Function) + method + "(" + strings.Join(arguments, ", ") + ")"
	case *BinaryExpression:
		return "(" + grouped(typed.Left) + " " + typed.Operator + " " + grouped(typed.Right) + ")"
	case *UnaryExpression:
		return "(" + typed.Operator + " " + grouped(typed.Operand) + ")"
	case *ParenthesizedExpression:
		return grouped(typed.Inner)
	case *TableLiteral:
		return "{}"
	default:
		return "<bad>"
	}
}

func TestOperatorPrecedenceAndAssociativity(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{content: "1 + 2 * 3", want: "(1 + (2 * 3))"},
		{content: "1 * 2 + 3", want: "((1 * 2) + 3)"},
		{content: "1 - 2 - 3", want: "((1 - 2) - 3)"},
		// Concatenation and exponentiation group from the right.
		{content: `"a" .. "b" .. "c"`, want: `("a" .. ("b" .. "c"))`},
		{content: "2 ^ 3 ^ 2", want: "(2 ^ (3 ^ 2))"},
		// Exponentiation binds tighter than the unary minus on its left.
		{content: "-x ^ 2", want: "(- (x ^ 2))"},
		{content: "not a == b", want: "((not a) == b)"},
		{content: "a or b and c", want: "(a or (b and c))"},
		{content: "a < b and b <= c", want: "((a < b) and (b <= c))"},
		{content: "#list + 1", want: "((# list) + 1)"},
		{content: "(1 + 2) * 3", want: "((1 + 2) * 3)"},
		{content: `a.b["c"].d`, want: `a["b"]["c"]["d"]`},
		{content: `msg.post("#sprite", hash("play"), { id = 1 })`, want: `msg["post"]("#sprite", hash("play"), {})`},
		{content: `self.sound:play "bounce"`, want: `self["sound"]:play("bounce")`},
		{content: "f{}", want: "f({})"},
	}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			chunk := parse(t, "return "+tt.content)
			returned := chunk.Body[0].(*ReturnStatement).Values[0]
			if got := grouped(returned); got != tt.want {
				t.Errorf("grouped as %s, want %s", got, tt.want)
			}
		})
	}
}

func TestStatements(t *testing.T) {
	chunk := parse(t, `
local speed, direction = 320, vmath.vector3(1, 0, 0)
local function clamp(value, low, high) return math.max(low, math.min(high, value)) end
function M.helper(...) end
function init(self)
    self.speed = speed
    for i = 1, 10, 2 do print(i) end
    for key, value in pairs(self) do print(key, value) end
    while self.speed > 0 do self.speed = self.speed - 1 end
    repeat self.speed = self.speed + 1 until self.speed >= 10
    do local scoped = true end
    if self.speed > 5 then return elseif self.speed > 2 then print("mid") else print("low") end
end
function Ball:serve(side) self.side = side; return self end
`)

	kinds := make([]string, 0, len(chunk.Body))
	for _, statement := range chunk.Body {
		kinds = append(kinds, strings.TrimPrefix(fmt.Sprintf("%T", statement), "*lua."))
	}
	want := "LocalStatement LocalFunctionStatement FunctionStatement FunctionStatement FunctionStatement"
	if got := strings.Join(kinds, " "); got != want {
		t.Errorf("statements = %s, want %s", got, want)
	}

	init := chunk.Body[3].(*FunctionStatement)
	if name := init.Name.(*Identifier).Name; name != "init" || len(init.Function.Parameters) != 1 {
		t.Errorf("init = %q with %d parameters", name, len(init.Function.Parameters))
	}
	if got := len(init.Function.Body); got != 7 {
		t.Errorf("init has %d statements, want 7", got)
	}

	serve := chunk.Body[4].(*FunctionStatement)
	if serve.Method == nil || serve.Method.Name != "serve" {
		t.Errorf("Ball:serve lost its method name: %+v", serve.Method)
	}
	if helper := chunk.Body[2].(*FunctionStatement); !helper.Function.IsVararg {
		t.Error("M.helper(...) is not marked as vararg")
	}
}

func TestPositionsCoverTheWholeNode(t *testing.T) {
	chunk := parse(t, "local url = \"#sprite\"\nmsg.post(url, \"enable\")\n")

	literal := chunk.Body[0].(*LocalStatement).Values[0].(*StringLiteral)
	if literal.Start.Line != 1 || literal.Start.Column != 13 || literal.End.Column != 22 {
		t.Errorf("string spans %d:%d-%d, want 1:13-22", literal.Start.Line, literal.Start.Column, literal.End.Column)
	}

	call := chunk.Body[1].(*CallStatement).Call
	if call.Start.Line != 2 || call.Start.Column != 1 || call.End.Column != 24 {
		t.Errorf("call spans %d:%d-%d, want 2:1-24", call.Start.Line, call.Start.Column, call.End.Column)
	}
}

func TestInspectVisitsEveryNode(t *testing.T) {
	chunk := parse(t, `
function on_message(self, message_id, message, sender)
    if message_id == hash("collision_response") then
        msg.post(sender, "hit", { damage = self.damage })
    end
end
`)

	var strings []string
	Inspect(chunk, func(node Node) bool {
		if literal, isString := node.(*StringLiteral); isString {
			strings = append(strings, literal.Value)
		}
		return true
	})

	want := []string{"collision_response", "post", "hit", "damage", "damage"}
	if fmt.Sprint(strings) != fmt.Sprint(want) {
		t.Errorf("string literals = %v, want %v", strings, want)
	}
}

// A script is broken most of the time it is being edited. The parser has to
// say what is wrong and still hand back everything it understood, so that the
// rest of the file keeps being checked.
func TestBrokenCodeStillYieldsTheStatementsAroundIt(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantError     string
		wantFunctions []string
	}{
		{
			name:          "call left open",
			content:       "function init(self)\n    msg.post(\"#sprite\", \nend\nfunction update(self, dt) end\n",
			wantError:     `unexpected "end"`,
			wantFunctions: []string{"init", "update"},
		},
		{
			name:      "missing end",
			content:   "function init(self)\n    if self.ready then\n        start()\nend\nfunction update(self, dt) end\n",
			wantError: `expected "end" to close the function that starts on line 1`,
			// The "end" meant for the function closes the "if" instead, so
			// update is read as nested inside init. It is still found.
			wantFunctions: []string{"init", "update"},
		},
		{
			name:          "half-typed assignment",
			content:       "function init(self)\n    self.speed =\nend\nfunction final(self) end\n",
			wantError:     `unexpected "end"`,
			wantFunctions: []string{"init", "final"},
		},
		{
			name:          "stray token between statements",
			content:       "function init(self) end\n)\nfunction update(self, dt) end\n",
			wantError:     `unexpected ")"`,
			wantFunctions: []string{"init", "update"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, errors := Parse([]byte(tt.content))

			if len(errors) == 0 || errors[0].Message != tt.wantError {
				t.Errorf("errors = %+v, want first to be %q", errors, tt.wantError)
			}
			var functions []string
			Inspect(chunk, func(node Node) bool {
				if function, isFunction := node.(*FunctionStatement); isFunction {
					if name, isPlainName := function.Name.(*Identifier); isPlainName {
						functions = append(functions, name.Name)
					}
				}
				return true
			})
			if fmt.Sprint(functions) != fmt.Sprint(tt.wantFunctions) {
				t.Errorf("functions = %v, want %v", functions, tt.wantFunctions)
			}
		})
	}
}

func TestParserAlwaysTerminates(t *testing.T) {
	for _, content := range []string{"", "(", ")", "}", "{", "end", "local", "function", "x =", "= =", "if", "for", "a.", "a:", "f(", "{{{", "::", "\"", "[[", "local function"} {
		t.Run(content, func(_ *testing.T) {
			Parse([]byte(content))
		})
	}
}
