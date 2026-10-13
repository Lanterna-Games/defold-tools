package textproto

import (
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/source"
)

func parse(t *testing.T, text string) *Message {
	t.Helper()
	message, errors := Parse([]byte(text), source.NewLocator([]byte(text)))
	for _, err := range errors {
		t.Errorf("unexpected syntax error: %v", err)
	}
	return message
}

func TestParseScalarsAndNestedMessages(t *testing.T) {
	message := parse(t, `name: "main"
scale_along_z: 0
instances {
  id: "ball"
  prototype: "/main/ball.go"
  position {
    x: 480.0
  }
}
instances {
  id: "game"
}
`)

	if got := message.Text("name"); got != "main" {
		t.Errorf("name = %q, want %q", got, "main")
	}
	if got := message.Text("scale_along_z"); got != "0" {
		t.Errorf("scale_along_z = %q, want %q", got, "0")
	}
	instances := message.Messages("instances")
	if len(instances) != 2 {
		t.Fatalf("got %d instances, want 2", len(instances))
	}
	if got := instances[0].Messages("position")[0].Text("x"); got != "480.0" {
		t.Errorf("position.x = %q, want %q", got, "480.0")
	}
}

func TestPositionsAreOneBasedLineAndByteColumn(t *testing.T) {
	message := parse(t, "name: \"main\"\ninstances {\n  id: \"ball\"\n}\n")

	id, _ := message.Messages("instances")[0].Scalar("id")
	if id.Position.Line != 3 || id.Position.Column != 7 {
		t.Errorf("id is at %d:%d, want 3:7", id.Position.Line, id.Position.Column)
	}
}

func TestQuotedScalarsAreUnescaped(t *testing.T) {
	message := parse(t, `text: "say \"hi\"\n\ttab \x41 \102 back\\slash"`)

	want := "say \"hi\"\n\ttab A B back\\slash"
	if got := message.Text("text"); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

// The Defold editor writes an embedded component as one string literal per
// line of the document it holds, and the literals are concatenated.
func TestAdjacentStringsAreConcatenated(t *testing.T) {
	message := parse(t, "data: \"first: 1\\n\"\n  \"second: 2\\n\"\n  \"\"\n")

	if got, want := message.Text("data"), "first: 1\nsecond: 2\n"; got != want {
		t.Errorf("data = %q, want %q", got, want)
	}
}

// A problem inside an embedded document has to be reported where the user
// can see it: in the outer file, on the line of the string literal that
// holds it, at the column after the escapes that come before it.
func TestEmbeddedDocumentPositionsPointIntoTheOuterFile(t *testing.T) {
	text := "embedded_components {\n" +
		"  id: \"sprite\"\n" +
		"  data: \"default_animation: \\\"ball\\\"\\n\"\n" +
		"  \"material: \\\"/builtins/materials/sprite.material\\\"\\n\"\n" +
		"}\n"
	message := parse(t, text)

	data, _ := message.Messages("embedded_components")[0].Scalar("data")
	embedded, errors := data.Embedded()
	if len(errors) > 0 {
		t.Fatalf("embedded document has syntax errors: %v", errors)
	}

	material, _ := embedded.Scalar("material")
	if material.Text != "/builtins/materials/sprite.material" {
		t.Errorf("material = %q", material.Text)
	}
	if material.Position.Line != 4 {
		t.Errorf("material is on line %d, want 4", material.Position.Line)
	}
	// Line 4 is `  "material: \"/builtins...`; the value's opening quote is the
	// backslash-quote pair that starts at column 14.
	if material.Position.Column != 14 {
		t.Errorf("material is at column %d, want 14", material.Position.Column)
	}
	if text[material.Position.Offset:material.Position.Offset+2] != `\"` {
		t.Errorf("offset points at %q, want the escaped quote", text[material.Position.Offset:material.Position.Offset+2])
	}
}

func TestListsRepeatTheField(t *testing.T) {
	message := parse(t, `children: ["a", "b"]`)

	children := message.Scalars("children")
	if len(children) != 2 || children[0].Text != "a" || children[1].Text != "b" {
		t.Errorf("children = %v, want a and b", children)
	}
}

func TestCommentsAreSkipped(t *testing.T) {
	message := parse(t, "# header\nname: \"main\" # trailing\n")

	if got := message.Text("name"); got != "main" {
		t.Errorf("name = %q, want %q", got, "main")
	}
}

// A file being typed is broken most of the time. What can be read is still
// returned, so that the rest of the project keeps being checked.
func TestBrokenInputStillYieldsWhatCouldBeRead(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantError string
		wantName  string
	}{
		{name: "missing closing brace", text: "name: \"main\"\ninstances {\n  id: \"ball\"\n", wantError: `missing "}"`, wantName: "main"},
		{name: "unclosed string", text: "name: \"main\"\nid: \"ball\n", wantError: "string is not closed", wantName: "main"},
		{name: "stray character", text: "name: \"main\"\n= oops\nid: \"x\"\n", wantError: `unexpected "="`, wantName: "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, errors := Parse([]byte(tt.text), source.NewLocator([]byte(tt.text)))

			if len(errors) == 0 || errors[0].Message != tt.wantError {
				t.Errorf("errors = %v, want first to be %q", errors, tt.wantError)
			}
			if got := message.Text("name"); got != tt.wantName {
				t.Errorf("name = %q, want %q", got, tt.wantName)
			}
		})
	}
}
