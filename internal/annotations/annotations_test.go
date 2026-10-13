package annotations

import (
	"slices"
	"strings"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

func value(name string, optional bool, types ...string) docs.Value {
	return docs.Value{Name: name, Types: types, Doc: "the <code>" + name + "</code>", IsOptional: optional}
}

func referenceForTests() *docs.Reference {
	return &docs.Reference{Version: "1.13.2", Modules: []docs.Module{
		{Namespace: "base", Functions: []docs.Function{{Name: "print"}}},
		{Namespace: "builtins", Functions: []docs.Function{{Name: "hash", Parameters: []docs.Value{value("s", false, "string")}, Returns: []docs.Value{value("hash", false, "hash")}}}},
		{Namespace: "editor", Functions: []docs.Function{{Name: "editor.get"}}},
		{Namespace: "editor.prefs", Functions: []docs.Function{{Name: "editor.prefs.get"}}},
		{
			Namespace:   "go",
			Description: "<p>Functions for game objects.</p>",
			Constants: []docs.Constant{
				{Name: "go.PLAYBACK_LOOP_FORWARD", Type: "go.PLAYBACK", Doc: "loop forward"},
				{Name: "go.EASING_LINEAR", Doc: "linear"},
				{Name: "other.CONSTANT", Doc: "not of this module"},
			},
			Types: []docs.Type{
				{Name: "go.PLAYBACK", Kind: docs.KindOfEnum, Doc: "Playback modes"},
				{Name: "go.property_options", Kind: docs.KindOfStruct, Fields: []docs.Value{value("index", true, "integer"), value("key", false, "hash")}},
				{Name: "go.id", Kind: docs.KindOfAlias, Fields: []docs.Value{value("", false, "string"), value("", false, "hash")}},
			},
			Functions: []docs.Function{
				{
					Name:        "go.animate",
					Description: "<p>Animates a property.</p>",
					Examples:    "<pre><code>go.animate(\".\", \"position.x\")\n</code></pre>",
					Parameters: []docs.Value{
						value("url", false, "string", "hash", "url"),
						value("playback", false, "constant"),
						value("to", false, "vmath.vector3", "number"),
						value("end", false, "bool"),
						value("delay", true, "number"),
						value("complete_function", true, "function(self, url, property)"),
						value("...", true, "any"),
					},
					Returns: []docs.Value{value("the id", false, "hash"), {Types: []string{"dmhash_t*"}}, {Types: []string{"number"}, Doc: "how many"}},
				},
				{Name: "go.sub.deep.call"},
				{Name: "client:send"},
				{Name: "outside.the_module"},
			},
		},
		{Namespace: "b2d.body", Functions: []docs.Function{{Name: "b2d.body.get_position", Returns: []docs.Value{value("position", false, "vector3")}}}},
		{Namespace: "empty"},
	}}
}

func TestOnlyWhatTheLanguageServerDoesNotKnowIsWritten(t *testing.T) {
	files := Generate(referenceForTests(), docs.Links{})

	for _, name := range []string{"go.lua", "builtins.lua", "b2d_body.lua", "defold.lua"} {
		if _, isWritten := files[name]; !isWritten {
			t.Errorf("%s was not written; files = %v", name, keysOf(files))
		}
	}
	for _, name := range []string{"base.lua", "editor.lua", "editor_prefs.lua", "empty.lua"} {
		if _, isWritten := files[name]; isWritten {
			t.Errorf("%s was written: the language server knows the standard library, the editor API is not for game scripts, and an empty module says nothing", name)
		}
	}
}

func keysOf(files map[string]string) []string {
	var keys []string
	for key := range files {
		keys = append(keys, key)
	}
	return keys
}

func TestAModuleIsWrittenAsTheLanguageServerReadsIt(t *testing.T) {
	written := Generate(referenceForTests(), docs.Links{})["go.lua"]

	for _, want := range []string{
		"---@meta\n",
		"---Functions for game objects.\n---@class defold.go\n",
		"---loop forward\n---\n---[defold.com](https://defold.com/ref/go-lua#go.PLAYBACK)\n---@field PLAYBACK_LOOP_FORWARD go.PLAYBACK\n",
		"---@field EASING_LINEAR constant\n",
		"go = {}\n",
		"---Playback modes\n---@class go.PLAYBACK\n",
		"---@class go.property_options\n---@field index? integer the `index`\n---@field key hash the `key`\n",
		"---@alias go.id string|hash\n",
		"---Animates a property.\n---\n---**Examples:**\n---\n---```lua\n---go.animate(\".\", \"position.x\")\n---```\n",
		"---@param url string|hash|url the `url`\n",
		"---@param playback constant the `playback`\n",
		"---@param to vector3|number the `to`\n",
		"---@param _end boolean the `end`\n",
		"---@param delay? number the `delay`\n",
		"---@param complete_function? fun(self, url, property) the `complete_function`\n",
		"---@param ... any the `...`\n",
		"---@return hash the_id the `the id`\n",
		"---@return any\n",
		"---@return number # how many\n",
		"---[defold.com](https://defold.com/ref/go-lua#go.animate:url-playback-to-end-delay-complete_function-...)\n",
		"function go.animate(url, playback, to, _end, delay, complete_function, ...) end\n",
		"go.sub = {}\n\ngo.sub.deep = {}\n\n",
		"function go.sub.deep.call() end\n",
	} {
		if !strings.Contains(written, want) {
			t.Errorf("go.lua lacks %q", want)
		}
	}
	for _, unwanted := range []string{"CONSTANT", "client", "outside"} {
		if strings.Contains(written, unwanted) {
			t.Errorf("go.lua holds %q, which is not of the module or cannot be declared", unwanted)
		}
	}
}

func TestFunctionsWithoutAModuleAreGlobals(t *testing.T) {
	written := Generate(referenceForTests(), docs.Links{})["builtins.lua"]

	if !strings.Contains(written, "---@return hash hash the `hash`\n") || !strings.Contains(written, "\nfunction hash(s) end\n") {
		t.Errorf("builtins.lua = %q, want hash declared as a global", written)
	}
	if strings.Contains(written, "builtins = {}") {
		t.Error("builtins.lua declares a table called builtins, which does not exist in the engine")
	}
}

// What a script may name without declaring it: each module by its first name,
// and each function that has no module. What the Lua language server already
// knows, and what only editor scripts have, stay out.
func TestTheGlobalsAreTheModulesAndTheFunctionsWithoutOne(t *testing.T) {
	globals := Globals(referenceForTests())

	if want := []string{"b2d", "go", "hash"}; !slices.Equal(globals, want) {
		t.Errorf("globals = %v, want %v", globals, want)
	}
}

func TestANestedModuleDoesNotRedefineItsParent(t *testing.T) {
	alone := Generate(referenceForTests(), docs.Links{})["b2d_body.lua"]
	if !strings.Contains(alone, "b2d = {}\n\n") || !strings.Contains(alone, "b2d.body = {}\n") {
		t.Errorf("b2d_body.lua = %q, want the parent defined since no module does it", alone)
	}

	withTheParent := referenceForTests()
	withTheParent.Modules = append([]docs.Module{{Namespace: "b2d", Functions: []docs.Function{{Name: "b2d.get_world"}}}}, withTheParent.Modules...)
	nested := Generate(withTheParent, docs.Links{})["b2d_body.lua"]
	if strings.Contains(nested, "b2d = {}") {
		t.Errorf("b2d_body.lua = %q, want the parent left to its own module", nested)
	}
}

func TestTheTypesTheDocumentationNamesAreDefined(t *testing.T) {
	written := Generate(referenceForTests(), docs.Links{})["defold.lua"]

	for _, want := range []string{"---@class vector3\n---@field x number\n", "---@class hash\n", "---@class url\n", "---@class constant\n"} {
		if !strings.Contains(written, want) {
			t.Errorf("defold.lua lacks %q", want)
		}
	}
	for _, unwanted := range []string{"---@class go.PLAYBACK", "---@class string", "---@class any", "---@class self", "---@class property\n"} {
		if strings.Contains(written, unwanted) {
			t.Errorf("defold.lua holds %q: it is defined by its module, is the language's own, or is the name of an argument", unwanted)
		}
	}
}

func TestLinksLeadToTheOfflineReferenceWhenThereIsOne(t *testing.T) {
	written := Generate(referenceForTests(), docs.Links{OfflineReference: "http://127.0.0.1:49200"})["go.lua"]

	want := "---[Offline reference](http://127.0.0.1:49200/1.13.2/go#go.animate) · [defold.com](https://defold.com/ref/go-lua#go.animate:"
	if !strings.Contains(written, want) {
		t.Errorf("go.lua lacks %q", want)
	}
}

func TestTypesAsOldDocumentationWritesThem(t *testing.T) {
	for written, want := range map[string]string{
		"":                                   "any",
		"string":                             "string",
		"bool":                               "boolean",
		"vmath.vector3":                      "vector3",
		"function(self, url, property)":      "fun(self, url, property)",
		"function()":                         "fun()",
		"function":                           "function",
		"table<string, any>|nil":             "table<string, any>|nil",
		"fun(self:script_instance, id:hash)": "fun(self:script_instance, id:hash)",
		"hash[]":                             "hash[]",
		"const char*":                        "any",
		"dmGameObject::HInstance":            "any",
		"table<string":                       "any",
	} {
		if got := normalised(written); got != want {
			t.Errorf("normalised(%q) = %q, want %q", written, got, want)
		}
	}
}
