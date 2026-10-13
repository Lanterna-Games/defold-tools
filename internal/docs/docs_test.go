package docs

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func archiveOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func moduleOf(t *testing.T, reference *Reference, namespace string) Module {
	t.Helper()
	for _, module := range reference.Modules {
		if module.Namespace == namespace {
			return module
		}
	}
	t.Fatalf("the reference has no module %q", namespace)
	return Module{}
}

// Before 1.11 the documentation did not say which language a file was for, wrote
// an optional parameter in brackets, and listed constants as variables.
func TestTheDocumentationAsItWasWrittenBeforeItStatedALanguage(t *testing.T) {
	archive := archiveOf(t, map[string]string{
		"doc/go_doc.json": `{"info": {"namespace": "go", "name": "Game object", "brief": "Game object API"}, "elements": [
			{"type": "FUNCTION", "name": "go.animate", "brief": "animates a property", "description": "Animates <code>property</code>.", "examples": "<pre><code>go.animate(url)</code></pre>",
			 "parameters": [{"name": "url", "doc": "the url", "types": ["string", "hash", "url"]}, {"name": "[delay]", "doc": "the delay", "types": ["number"]}],
			 "returnvalues": [{"name": "id", "doc": "the id", "types": ["hash"]}]},
			{"type": "VARIABLE", "name": "go.PLAYBACK_LOOP_FORWARD", "brief": "loop forward"},
			{"type": "MESSAGE", "name": "set_parent", "brief": "sets the parent", "parameters": [{"name": "parent_id", "doc": "the parent", "types": ["hash"]}]},
			{"type": "PROPERTY", "name": "position", "brief": "the position"}]}`,
		"doc/dmGameObject_doc.json":  `{"info": {"namespace": "dmGameObject", "name": "Game object SDK"}, "elements": [{"type": "FUNCTION", "name": "dmGameObject::New"}]}`,
		"doc/sharedlibrary_doc.json": `{"info": {"namespace": "sharedlibrary", "name": "Shared library"}, "elements": [{"type": "MACRO", "name": "DM_DLLEXPORT"}]}`,
		"doc/engine_doc.json":        `{"info": {"namespace": "engine", "name": "Engine"}, "elements": [{"type": "FUNCTION", "name": "Engine::Step"}]}`,
		"doc/go.script_api":          "not json",
	})

	reference, err := FromArchive("1.6.0", archive)
	if err != nil {
		t.Fatal(err)
	}

	if len(reference.Modules) != 1 {
		t.Fatalf("modules = %d, want only the one for scripts: %+v", len(reference.Modules), reference.Modules)
	}
	module := moduleOf(t, reference, "go")
	animate := module.Functions[0]
	if animate.Name != "go.animate" || len(animate.Parameters) != 2 || len(animate.Returns) != 1 {
		t.Fatalf("function = %+v", animate)
	}
	if delay := animate.Parameters[1]; delay.Name != "delay" || !delay.IsOptional || animate.Parameters[0].IsOptional {
		t.Errorf("parameters = %+v, want the one in brackets optional and without them", animate.Parameters)
	}
	if len(module.Constants) != 1 || module.Constants[0].Name != "go.PLAYBACK_LOOP_FORWARD" || module.Constants[0].Doc != "loop forward" {
		t.Errorf("constants = %+v", module.Constants)
	}
	if len(module.Messages) != 1 || module.Messages[0].Fields[0].Name != "parent_id" || len(module.Properties) != 1 {
		t.Errorf("messages = %+v, properties = %+v", module.Messages, module.Properties)
	}
}

func TestTheDocumentationAsItIsWrittenNow(t *testing.T) {
	archive := archiveOf(t, map[string]string{
		"doc/gameobject_script.cpp_doc.json": `{"info": {"namespace": "go", "name": "Game object", "language": "Lua"}, "elements": [
			{"type": "FUNCTION", "name": "go.animate", "parameters": [
				{"name": "playback", "types": ["go.PLAYBACK"], "is_optional": "False"},
				{"name": "delay", "types": ["number"], "is_optional": "True"}]},
			{"type": "ENUM", "name": "go.PLAYBACK", "brief": "Playback modes", "members": [{"name": "go.PLAYBACK_LOOP_FORWARD", "doc": "loop forward", "type": ""}]},
			{"type": "STRUCT", "name": "go.property_options", "brief": "Property access options", "members": [{"name": "index?", "doc": "Index into an array", "type": "integer"}, {"name": "key", "doc": "Name", "type": "hash"}]}]}`,
		"doc/gameobject_ddf.proto_doc.json": `{"info": {"namespace": "go", "name": "Game object", "language": "Lua"}, "elements": [{"type": "MESSAGE", "name": "enable"}]}`,
		"doc/dmsdk-gameobject_doc.json":     `{"info": {"namespace": "go", "name": "Game object", "language": "C++"}, "elements": [{"type": "FUNCTION", "name": "New"}]}`,
	})

	reference, err := FromArchive("1.13.2", archive)
	if err != nil {
		t.Fatal(err)
	}

	module := moduleOf(t, reference, "go")
	if len(module.Functions) != 1 || len(module.Messages) != 1 {
		t.Fatalf("module = %+v, want the two files for scripts merged and the C++ one left out", module)
	}
	if parameters := module.Functions[0].Parameters; parameters[0].IsOptional || !parameters[1].IsOptional {
		t.Errorf("parameters = %+v, want only the delay optional", parameters)
	}
	if len(module.Constants) != 1 || module.Constants[0].Type != "go.PLAYBACK" {
		t.Errorf("constants = %+v, want the member of the enum, typed by it", module.Constants)
	}
	if len(module.Types) != 2 || module.Types[0].Kind != KindOfEnum || module.Types[1].Kind != KindOfStruct {
		t.Fatalf("types = %+v", module.Types)
	}
	if fields := module.Types[1].Fields; fields[0].Name != "index" || !fields[0].IsOptional || fields[1].IsOptional {
		t.Errorf("fields = %+v, want the one marked ? optional and without the mark", fields)
	}
}

func TestAnArchiveWithNothingForScriptsIsRefused(t *testing.T) {
	archive := archiveOf(t, map[string]string{"doc/dmGameObject_doc.json": `{"info": {"namespace": "dmGameObject"}, "elements": [{"type": "FUNCTION", "name": "dmGameObject::New"}]}`})

	if _, err := FromArchive("0.1.0", archive); err == nil || !strings.Contains(err.Error(), "holds nothing for scripts") {
		t.Errorf("error = %v, want a refusal", err)
	}
	if _, err := FromArchive("0.1.0", []byte("not a zip")); err == nil {
		t.Error("something that is not an archive was read")
	}
}

func TestMarkdown(t *testing.T) {
	cases := map[string]struct {
		markup string
		want   string
	}{
		"plain text":                         {markup: "Posts a message.", want: "Posts a message."},
		"paragraphs":                         {markup: "<p>One.</p>\n<p>Two\nlines.</p>", want: "One.\n\nTwo lines."},
		"inline code":                        {markup: "The <code>\"#\"</code> shorthand &amp; more", want: "The `\"#\"` shorthand & more"},
		"emphasis":                           {markup: "<strong>Note</strong> the <em>order</em>", want: "**Note** the *order*"},
		"a code block":                       {markup: "Send it:\n<div class=\"codehilite\"><pre><span></span><code><span class=\"n\">msg</span><span class=\"p\">.</span>post(&quot;#&quot;)\n</code></pre></div>\nDone.", want: "Send it:\n\n```lua\nmsg.post(\"#\")\n```\n\nDone."},
		"a list":                             {markup: "Shorthands:\n<ul>\n<li><code>\".\"</code> the game object</li>\n<li><code>\"#\"</code> the component</li>\n</ul>\nThere is a limit.", want: "Shorthands:\n\n- `\".\"` the game object\n- `\"#\"` the component\n\nThere is a limit."},
		"a link that only works on its page": {markup: "See <a href=\"#on_input\">on_input</a> for more.", want: "See on_input for more."},
		"a link":                             {markup: "See the <a href=\"/manuals/animation#_easing\">animation guide</a> or <a href=\"https://example.com\">this</a>.", want: "See the [animation guide](https://defold.com/manuals/animation#_easing) or [this](https://example.com)."},
		"an anchor":                          {markup: "<a name=\"here\">Here</a> it is", want: "Here it is"},
		"an icon":                            {markup: "<span class=\"icon-attention\"></span> Careful<br/>now", want: "Careful\nnow"},
		"nothing":                            {markup: "  \n ", want: ""},
	}
	for name, testCase := range cases {
		if got := Markdown(testCase.markup); got != testCase.want {
			t.Errorf("%s: Markdown = %q, want %q", name, got, testCase.want)
		}
	}
	if got := OneLine("<p>One.</p>\n<ul><li>a</li><li>b</li></ul>"); got != "One. - a - b" {
		t.Errorf("OneLine = %q", got)
	}
}
