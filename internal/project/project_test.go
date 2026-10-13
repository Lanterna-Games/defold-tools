package project

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
)

func load(t *testing.T, files map[string]string) *Project {
	t.Helper()
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	loaded, err := Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func TestGameObjectListsFileAndEmbeddedComponents(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/ball.go": `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"ball\"\n"
  ""
}
`,
	})

	ball := loaded.GameObjects["/main/ball.go"]
	if ball == nil {
		t.Fatal("ball.go was not indexed")
	}
	script, _ := ball.Component("ball")
	if script.Type != "script" || script.Resource != "/main/ball.script" {
		t.Errorf("script component = %+v", script)
	}
	sprite, found := ball.Component("sprite")
	if !found || sprite.Type != "sprite" {
		t.Errorf("sprite component = %+v, found = %v", sprite, found)
	}
	if sprite.Location.Position.Line != 6 {
		t.Errorf("sprite is reported on line %d, want the line of its id, 6", sprite.Location.Position.Line)
	}
}

func TestCollectionListsInstancesOverridesAndChildren(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/main.collection": `name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
  children: "shadow"
  component_properties {
    id: "ball"
    properties {
      id: "game"
      value: "/game#game"
      type: PROPERTY_TYPE_URL
    }
  }
}
embedded_instances {
  id: "shadow"
  data: "embedded_components {\n"
  "  id: \"sprite\"\n"
  "  type: \"sprite\"\n"
  "  data: \"default_animation: \\\"shadow\\\"\\n\"\n"
  "  \"\"\n"
  "}\n"
  ""
}
`,
	})

	main := loaded.Collections["/main/main.collection"]
	if main.Name != "main" || len(main.Instances) != 2 {
		t.Fatalf("collection = %+v", main)
	}

	ball := main.Instances[0]
	if ball.Prototype != "/main/ball.go" || len(ball.Children) != 1 || ball.Children[0].ID != "shadow" {
		t.Errorf("ball = %+v", ball)
	}
	override := ball.PropertyOverrides[0]
	if override.Component != "ball" || override.Property != "game" || override.Value != "/game#game" || override.Type != PropertyTypeURL {
		t.Errorf("override = %+v", override)
	}
	if override.Location.Position.Line != 10 {
		t.Errorf("override is reported on line %d, want the line of its value, 10", override.Location.Position.Line)
	}

	shadow := main.Instances[1]
	if _, found := shadow.EmbeddedObject.Component("sprite"); !found {
		t.Errorf("the embedded game object lost its sprite: %+v", shadow.EmbeddedObject)
	}
}

func TestResourceReferencesReachIntoEmbeddedDocuments(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/ball.go": `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "textures {\n"
  "  sampler: \"texture_sampler\"\n"
  "  texture: \"/assets/pong.atlas\"\n"
  "}\n"
  ""
}
`,
	})

	got := map[string]int{}
	for _, reference := range loaded.References {
		got[reference.Path] = reference.Location.Position.Line
	}
	if got["/main/ball.script"] != 3 {
		t.Errorf("script reference on line %d, want 3", got["/main/ball.script"])
	}
	if got["/assets/pong.atlas"] != 10 {
		t.Errorf("atlas reference on line %d, want 10: the line of the string literal that holds it", got["/assets/pong.atlas"])
	}
}

// A label may hold any text, and a property value may be a URL: neither is a file.
func TestFreeTextIsNotMistakenForAResource(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/hud.gui": `nodes {
  id: "/not/a.file"
  text: "/also/not.a.file"
}
`,
	})

	if len(loaded.References) != 0 {
		t.Errorf("references = %+v, want none", loaded.References)
	}
}

func TestBuildOutputAndHiddenDirectoriesAreNotRead(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/ball.go":            "",
		"build/default/copy.go":   "",
		".internal/lib/cached.go": "",
		"main/build/level.go":     "",
	})

	if len(loaded.GameObjects) != 2 {
		t.Errorf("indexed %v, want only main/ball.go and main/build/level.go", sortedKeys(loaded.GameObjects))
	}
}

func TestMountsFlattenNestedCollectionsUnderTheirRoot(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/main.collection": `name: "main"
instances {
  id: "game"
  prototype: "/main/game.go"
}
collection_instances {
  id: "level"
  collection: "/main/level.collection"
}
`,
		"main/level.collection": `name: "level"
instances {
  id: "door"
  prototype: "/main/door.go"
}
`,
		"main/game.go": "",
		"main/door.go": "",
	})

	mounts := loaded.Mounts()
	if len(mounts) != 2 {
		t.Fatalf("got %d mounts, want the root and the nested collection", len(mounts))
	}
	if mounts[1].Prefix != "/level" || mounts[1].Collection.Name != "level" {
		t.Errorf("nested mount = %+v", mounts[1])
	}
	want := []string{"/game", "/level/door"}
	got := mounts[0].Namespace.GameObjectPaths()
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("namespace = %v, want %v", got, want)
	}
}

func TestACollectionThatContainsItselfDoesNotLoopForever(t *testing.T) {
	loaded := load(t, map[string]string{
		"a.collection": `name: "a"
collection_instances {
  id: "b"
  collection: "/b.collection"
}
`,
		"b.collection": `name: "b"
collection_instances {
  id: "a"
  collection: "/a.collection"
}
`,
	})

	// Neither is a root, because each is nested in the other.
	if mounts := loaded.Mounts(); len(mounts) != 0 {
		t.Errorf("got %d mounts, want none", len(mounts))
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		text string
		want URL
	}{
		{text: "#sprite", want: URL{Fragment: "sprite", HasFragment: true}},
		{text: "/game#game", want: URL{Path: "/game", Fragment: "game", HasFragment: true}},
		{text: "main:/level/door", want: URL{Socket: "main", Path: "/level/door"}},
		{text: "enemy", want: URL{Path: "enemy"}},
		{text: ".", want: URL{Path: "."}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := ParseURL(tt.text); got != tt.want {
				t.Errorf("ParseURL(%q) = %+v, want %+v", tt.text, got, tt.want)
			}
		})
	}
}

func scriptOf(t *testing.T, content string) *Script {
	t.Helper()
	return load(t, map[string]string{"main/game.script": content}).Scripts["/main/game.script"]
}

func TestScriptListsTheMessagesItPosts(t *testing.T) {
	script := scriptOf(t, `local SERVE = hash("serve")
local target = "/ball"
target = "/paddle"

function init(self)
    msg.post("#hud", "set_score", { left = 0 })
    msg.post(self.ball, SERVE)
    msg.post(target, hash("stop"))
    msg.post(".", "ball_" .. self.side)
end
`)

	want := []MessagePost{
		{ID: MessageID{Name: "set_score"}, IDIsKnown: true, Receiver: "#hud", ReceiverIsWritten: true, Fields: []string{"left"}, FieldsAreWritten: true},
		{ID: MessageID{Name: "serve"}, IDIsKnown: true, FieldsAreWritten: true},
		{ID: MessageID{Name: "stop"}, IDIsKnown: true, FieldsAreWritten: true},
		{Receiver: ".", ReceiverIsWritten: true, FieldsAreWritten: true},
	}
	if len(script.Posts) != len(want) {
		t.Fatalf("got %d posts, want %d: %+v", len(script.Posts), len(want), script.Posts)
	}
	for index, post := range script.Posts {
		post.ID.Location = Location{}
		if !reflect.DeepEqual(post, want[index]) {
			t.Errorf("post %d = %+v, want %+v", index, post, want[index])
		}
	}
	if line := script.Posts[1].ID.Location.Position.Line; line != 7 {
		t.Errorf("a post through a constant is located at line %d, want 7, where it is posted", line)
	}
}

// Only a name that stands for one id through the whole file is read as that id.
func TestANameThatDoesNotStandForOneIDIsNotResolved(t *testing.T) {
	scripts := map[string]string{
		"assigned again": `local ID = hash("a")
ID = hash("b")
msg.post(".", ID)`,
		"declared twice": `local ID = hash("a")
local function other() local ID = hash("b") end
msg.post(".", ID)`,
		"declared inside a function": `function init(self)
    local ID = hash("a")
    msg.post(".", ID)
end`,
		"built at run time": `local ID = hash("a" .. suffix)
msg.post(".", ID)`,
	}
	for name, content := range scripts {
		t.Run(name, func(t *testing.T) {
			script := scriptOf(t, content)

			if len(script.Posts) != 1 || script.Posts[0].IDIsKnown {
				t.Errorf("posts = %+v, want one post whose id is not known", script.Posts)
			}
		})
	}
}

func TestAMentionOfMsgPostThatIsNotACallCountsAsAPostThatCannotBeRead(t *testing.T) {
	script := scriptOf(t, `local post = msg.post
post(".", "serve")
`)

	if len(script.Posts) != 1 || script.Posts[0].IDIsKnown {
		t.Errorf("posts = %+v, want one post whose id is not known", script.Posts)
	}
}

func TestScriptListsTheMessagesItHandles(t *testing.T) {
	script := scriptOf(t, `local SERVE = hash("serve")

function on_message(self, message_id, message, sender)
    if message_id ~= SERVE and message_id ~= hash("stop") then
        return
    end
    if message_id == SERVE and message.direction then
        self.direction = message.direction
    elseif (hash("stop") == message_id) then
        self.direction = nil
    end
    if message.loud then
        print("loud")
    else
        print("quiet")
    end
end
`)

	if got := script.HandledNames(); len(got) != 2 || got[0] != "serve" || got[1] != "stop" {
		t.Errorf("handled = %v, want serve and stop", got)
	}
	if script.HandlesMessagesItDoesNotName {
		t.Error("the handler names every message it handles, and was taken to handle others")
	}
	if !script.Handles("serve") || script.Handles("start") {
		t.Error("Handles does not agree with the handler")
	}
}

func TestHandlersOfOtherFunctionsAreNotTheScriptsHandler(t *testing.T) {
	script := scriptOf(t, `local M = {}

function M.on_message(self, message_id)
    if message_id == hash("serve") then return true end
end

function M:on_message(message_id)
    if message_id == hash("stop") then return true end
end
`)

	if len(script.Handled) != 0 || script.HandlesMessagesItDoesNotName {
		t.Errorf("handled = %v, open-ended = %v; want a script with no handler", script.HandledNames(), script.HandlesMessagesItDoesNotName)
	}
}

func TestGUISceneNamesItsScript(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/game.go": `components {
  id: "hud"
  component: "/main/hud.gui"
}
components {
  id: "menu"
  component: "/main/menu.gui"
}
components {
  id: "shop"
  component: "/library/shop.gui"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: ""
}
`,
		"main/hud.gui":        "script: \"/main/hud.gui_script\"\n",
		"main/hud.gui_script": "",
		"main/menu.gui":       "script: \"\"\n",
	})
	gameObject := loaded.GameObjects["/main/game.go"]

	cases := map[string]struct {
		wantScript string
		wantSight  bool
	}{
		"hud":    {wantScript: "/main/hud.gui_script", wantSight: true},
		"menu":   {wantSight: true},
		"shop":   {wantSight: false},
		"sprite": {wantSight: true},
	}
	for id, want := range cases {
		component, _ := gameObject.Component(id)
		script, isInSight := loaded.ScriptRunBy(component)
		gotScript := ""
		if script != nil {
			gotScript = script.Path
		}
		if gotScript != want.wantScript || isInSight != want.wantSight {
			t.Errorf("%s: script %q in sight %v, want %q %v", id, gotScript, isInSight, want.wantScript, want.wantSight)
		}
	}
	if paths := loaded.ScriptPathsOf(gameObject); len(paths) != 1 || paths[0] != "/main/hud.gui_script" {
		t.Errorf("script paths = %v, want the script of the hud", paths)
	}
}

func TestProjectKnowsWhenCodeItCannotReadIsPartOfIt(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  bool
	}{
		"no settings":       {files: map[string]string{"main/game.script": ""}, want: false},
		"no dependencies":   {files: map[string]string{"game.project": "[project]\ntitle = Pong\n"}, want: false},
		"empty dependency":  {files: map[string]string{"game.project": "[project]\ndependencies#0 =\n"}, want: false},
		"other section":     {files: map[string]string{"game.project": "[library]\ndependencies = x\n"}, want: false},
		"one dependency":    {files: map[string]string{"game.project": "[bootstrap]\nmain = x\n\n[project]\ndependencies#0 = https://example.com/a.zip\n"}, want: true},
		"old single value":  {files: map[string]string{"game.project": "[project]\ndependencies = https://example.com/a.zip\n"}, want: true},
		"native extension":  {files: map[string]string{"native/ext.manifest": "name: \"native\"\n"}, want: true},
		"a library itself":  {files: map[string]string{"game.project": "[library]\ninclude_dirs = monarch\n"}, want: true},
		"not a library":     {files: map[string]string{"game.project": "[library]\ninclude_dirs =\n"}, want: false},
		"windows line ends": {files: map[string]string{"game.project": "[project]\r\ndependencies#0 = https://example.com/a.zip\r\n"}, want: true},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := load(t, testCase.files).RunsCodeOutOfSight; got != testCase.want {
				t.Errorf("RunsCodeOutOfSight = %v, want %v", got, testCase.want)
			}
		})
	}
}

func argumentOfUse(t *testing.T, script *Script) lua.Expression {
	t.Helper()
	var argument lua.Expression
	lua.Inspect(script.Chunk, func(node lua.Node) bool {
		if arguments, isUse := argumentsOf(asExpression(node), "use"); isUse && len(arguments) == 1 {
			argument = arguments[0]
		}
		return true
	})
	if argument == nil {
		t.Fatal("the script does not call use(...)")
	}
	return argument
}

func asExpression(node lua.Node) lua.Expression {
	expression, _ := node.(lua.Expression)
	return expression
}

func TestAddressWrittenIn(t *testing.T) {
	const declarations = `local HUD = "/hud#hud"
local LOADER = msg.url("main:/loader#loader")
local SKY = hash("/sky")
local moving = "/a"
moving = "/b"
`
	cases := map[string]struct {
		expression string
		want       string
		wantLine   int
	}{
		"a string":                      {expression: `"#sprite"`, want: "#sprite", wantLine: 6},
		"a string in parentheses":       {expression: `("#sprite")`, want: "#sprite", wantLine: 6},
		"a constant":                    {expression: `HUD`, want: "/hud#hud", wantLine: 1},
		"a constant holding a URL":      {expression: `LOADER`, want: "main:/loader#loader", wantLine: 2},
		"a constant holding a hash":     {expression: `SKY`, want: "/sky", wantLine: 3},
		"a URL":                         {expression: `msg.url("/hud#hud")`, want: "/hud#hud", wantLine: 6},
		"a URL in three parts":          {expression: `msg.url("main", "/hud", "hud")`, want: "main:/hud#hud", wantLine: 6},
		"a URL with parts left out":     {expression: `msg.url(nil, nil, "sprite")`, want: "#sprite", wantLine: 6},
		"a URL with hashed parts":       {expression: `msg.url(nil, hash("/hud"), hash("hud"))`, want: "/hud#hud", wantLine: 6},
		"the hash of a path":            {expression: `hash("/hud")`, want: "/hud", wantLine: 6},
		"the hash of a name":            {expression: `hash("hud")`},
		"the hash of a whole URL":       {expression: `hash("/hud#hud")`},
		"a name assigned twice":         {expression: `moving`},
		"a name from somewhere else":    {expression: `self.hud`},
		"a string built at run time":    {expression: `"/enemy" .. index`},
		"a URL of the script itself":    {expression: `msg.url()`},
		"a URL from a value":            {expression: `msg.url(path)`},
		"a URL with a part from a call": {expression: `msg.url(nil, go.get_id("hud"), "hud")`},
		"a URL in two parts":            {expression: `msg.url("/hud", "hud")`},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			script := scriptOf(t, declarations+"use("+testCase.expression+")\n")

			address, isWritten := script.AddressWrittenIn(argumentOfUse(t, script))

			if isWritten != (testCase.want != "") || (isWritten && address.Text != testCase.want) {
				t.Fatalf("address = %q, written = %v; want %q", address.Text, isWritten, testCase.want)
			}
			if isWritten && address.Location.Position.Line != testCase.wantLine {
				t.Errorf("located at line %d, want %d, where the text is", address.Location.Position.Line, testCase.wantLine)
			}
		})
	}
}

func TestScriptListsTheURLsItBuilds(t *testing.T) {
	script := scriptOf(t, `function init(self)
    self.sprites = { msg.url("#idle"), msg.url(nil, nil, "jump") }
    self.me = msg.url()
    self.other = msg.url(self.path)
end
`)

	built := script.URLsBuilt()

	if len(built) != 2 || built[0].Text != "#idle" || built[1].Text != "#jump" {
		t.Errorf("built = %+v, want #idle and #jump", built)
	}
}

func TestNamespacesAreFoundByTheNameOfTheirRoot(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/main.collection":  "name: \"main\"\ncollection_instances {\n  id: \"hud\"\n  collection: \"/main/hud.collection\"\n}\n",
		"main/hud.collection":   "name: \"hud\"\n",
		"levels/one.collection": "name: \"level\"\n",
		"levels/two.collection": "name: \"level\"\n",
	})

	for name, want := range map[string]int{"main": 1, "level": 2, "hud": 0, "nowhere": 0} {
		if got := len(loaded.NamespacesNamed(name)); got != want {
			t.Errorf("%d namespaces named %q, want %d", got, name, want)
		}
	}
	if first, again := loaded.Mounts(), loaded.Mounts(); first[0].Namespace != again[0].Namespace {
		t.Error("Mounts built the namespaces twice")
	}
}

func TestGameBindingComesFromTheSettings(t *testing.T) {
	cases := map[string]struct {
		settings string
		want     string
	}{
		"not set":         {settings: "[project]\ntitle = Pong\n", want: "/input/game.input_binding"},
		"set":             {settings: "[input]\ngame_binding = /controls/pong.input_bindingc\n", want: "/controls/pong.input_binding"},
		"the builtin one": {settings: "[input]\r\ngame_binding = /builtins/input/all.input_bindingc\r\n", want: "/builtins/input/all.input_binding"},
		"another section": {settings: "[display]\ngame_binding = /x.input_bindingc\n", want: "/input/game.input_binding"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := load(t, map[string]string{"game.project": testCase.settings}).GameBinding; got != testCase.want {
				t.Errorf("GameBinding = %q, want %q", got, testCase.want)
			}
		})
	}
	if got := load(t, map[string]string{"main/game.script": ""}).GameBinding; got != "/input/game.input_binding" {
		t.Errorf("GameBinding without settings = %q, want the default", got)
	}
}

func TestProjectListsTheActionsOfEachBinding(t *testing.T) {
	loaded := load(t, map[string]string{
		"input/game.input_binding": "key_trigger {\n  input: KEY_SPACE\n  action: \"jump\"\n}\nmouse_trigger {\n  input: MOUSE_BUTTON_1\n  action: \"touch\"\n}\n",
	})

	actions := loaded.InputBindings["/input/game.input_binding"]
	if len(actions) != 2 || actions[0] != "jump" || actions[1] != "touch" {
		t.Errorf("actions = %v, want jump and touch", actions)
	}
}

func TestScriptListsTheActionsItComparesAndFeedsItself(t *testing.T) {
	script := scriptOf(t, `local FIRE = hash("fire")

function on_input(self, action_id, action)
    if action_id == hash("jump") or FIRE == action_id then
        return true
    end
    return action_id ~= hash("pause") and action_id ~= self.ignored
end

function init(self)
    on_input(self, hash("start"), {})
end
`)

	var compared []string
	for _, action := range script.ActionsCompared {
		compared = append(compared, action.Name)
	}
	if len(compared) != 3 || compared[0] != "jump" || compared[1] != "fire" || compared[2] != "pause" {
		t.Errorf("compared = %v, want jump, fire and pause", compared)
	}
	if line := script.ActionsCompared[1].Location.Position.Line; line != 4 {
		t.Errorf("an action compared through a constant is located at line %d, want 4, where it is compared", line)
	}
	if len(script.ActionsItFeedsItself) != 1 || script.ActionsItFeedsItself[0] != "start" || script.FeedsItselfActionsThatCannotBeRead {
		t.Errorf("fed = %v, unreadable = %v; want start and false", script.ActionsItFeedsItself, script.FeedsItselfActionsThatCannotBeRead)
	}
}

func TestProjectListsCollisionGroupsAndMasks(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/wall.go": `embedded_components {
  id: "collisionobject"
  type: "collisionobject"
  data: "group: \"wall\"\n"
  "mask: \"ball\"\n"
  ""
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "group: \"not_a_collision_object\"\n"
}
`,
		"main/main.collection": `name: "main"
embedded_instances {
  id: "paddle"
  data: "embedded_components {\n"
  "  id: \"collisionobject\"\n"
  "  type: \"collisionobject\"\n"
  "  data: \"group: \\\"paddle\\\"\\n"
  "mask: \\\"ball\\\"\\n"
  "\"\n"
  "}\n"
  ""
}
`,
		"main/trigger.collisionobject": "group: \"trigger\"\nmask: \"player\"\n",
		"main/level.tilesource":        "collision_groups: \"ground\"\n",
	})

	want := []string{"ground", "paddle", "trigger", "wall"}
	if got := loaded.CollisionGroupNames(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Errorf("groups = %v, want %v", got, want)
	}
	if len(loaded.CollisionMasks) != 3 {
		t.Fatalf("masks = %+v, want three", loaded.CollisionMasks)
	}
	for _, mask := range loaded.CollisionMasks {
		if mask.Location.File == "main/wall.go" && mask.Location.Position.Line != 5 {
			t.Errorf("the mask of the wall is located at line %d, want 5, inside the embedded component", mask.Location.Position.Line)
		}
	}
}

func TestNodesOfASceneIncludeThoseOfItsTemplates(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/hud.gui": `nodes {
  id: "score"
}
nodes {
  id: "ok"
  template: "/main/button.gui"
}
nodes {
  id: "shop"
  template: "/library/shop.gui"
}
`,
		"main/button.gui": `nodes {
  id: "label"
}
nodes {
  id: "icon"
  template: "/main/icon.gui"
}
`,
		"main/icon.gui": `nodes {
  id: "image"
}
nodes {
  id: "again"
  template: "/main/button.gui"
}
`,
	})

	nodes := loaded.NodesOf(loaded.GUIScenes["/main/hud.gui"])

	for _, id := range []string{"score", "ok", "ok/label", "ok/icon", "ok/icon/image", "ok/icon/again", "shop"} {
		if !nodes.Has(id) {
			t.Errorf("%q is a node of the scene, and was not found among %v", id, nodes.IDs())
		}
	}
	for _, id := range []string{"label", "ok/icon/again/label", "shop/price"} {
		if nodes.Has(id) {
			t.Errorf("%q is not a node that was read, and was found", id)
		}
	}
	if !nodes.CouldHoldMoreThanWasRead("shop/price") || nodes.CouldHoldMoreThanWasRead("ok/price") || nodes.CouldHoldMoreThanWasRead("shopping") {
		t.Error("only what is under the template that is not in the project may be there unseen")
	}
}

func TestScriptListsTheFieldsItPosts(t *testing.T) {
	script := scriptOf(t, `local EMPTY = {}

function init(self)
    msg.post(".", "a", { x = 1, ["y"] = 2 })
    msg.post(".", "b")
    msg.post(".", "c", EMPTY)
    msg.post(".", "d", self.payload)
    msg.post(".", "e", { 1, 2 })
end
`)

	want := []struct {
		fields     []string
		areWritten bool
	}{
		{fields: []string{"x", "y"}, areWritten: true},
		{areWritten: true},
		{areWritten: true},
		{areWritten: false},
		{areWritten: false},
	}
	if len(script.Posts) != len(want) {
		t.Fatalf("got %d posts, want %d", len(script.Posts), len(want))
	}
	for index, post := range script.Posts {
		if !reflect.DeepEqual(post.Fields, want[index].fields) || post.FieldsAreWritten != want[index].areWritten {
			t.Errorf("post %q: fields %v, written %v; want %v, %v", post.ID.Name, post.Fields, post.FieldsAreWritten, want[index].fields, want[index].areWritten)
		}
	}
}

const idsModule = `local M = {}

local SERVE = hash("serve")

M.BALL_OUT = hash("ball_out")
M.SERVE = SERVE
M.HUD = "/hud#hud"
M.CHANGING = hash("first")
M.SOLID = { hash("wall"), hash("paddle") }

function M.rename()
    M.CHANGING = hash("second")
end

return M
`

func TestIDsAreFollowedIntoTablesAndRequiredModules(t *testing.T) {
	cases := map[string]struct {
		expression string
		want       string
	}{
		"a field of a required module":          {expression: `ids.BALL_OUT`, want: "ball_out"},
		"a field that holds a local constant":   {expression: `ids.SERVE`, want: "serve"},
		"a module required where it is used":    {expression: `require("main.ids").BALL_OUT`, want: "ball_out"},
		"a module required with another syntax": {expression: `other.BALL_OUT`, want: "ball_out"},
		"a field of a table of the file":        {expression: `LOCAL.START`, want: "start"},
		"a field set after the table":           {expression: `LOCAL.STOP`, want: "stop"},
		"a constant of a constant":              {expression: `ALIAS`, want: "ball_out"},
		"a field set more than once":            {expression: `ids.CHANGING`},
		"a field of the file set twice":         {expression: `LOCAL.TWICE`},
		"a field set inside a function":         {expression: `LOCAL.LATE`},
		"a field the module does not have":      {expression: `ids.NOTHING`},
		"a module that is not in the project":   {expression: `missing.BALL_OUT`},
		"a table that is not a constant":        {expression: `changing.START`},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			loaded := load(t, map[string]string{
				"main/ids.lua": idsModule,
				"main/game.script": `local ids = require("main.ids")
local other = require "main.ids"
local missing = require("library.ids")
local ALIAS = ids.BALL_OUT
local LOCAL = { START = hash("start"), TWICE = hash("one") }
LOCAL.STOP = "stop"
LOCAL.TWICE = hash("two")
local changing = { START = hash("start") }
changing = {}

function init(self)
    LOCAL.LATE = hash("late")
    use(` + testCase.expression + `)
end
`,
			})
			script := loaded.Scripts["/main/game.script"]

			got, isWritten := script.IDWrittenIn(argumentOfUse(t, script))

			if isWritten != (testCase.want != "") || got != testCase.want {
				t.Errorf("id = %q, written = %v; want %q", got, isWritten, testCase.want)
			}
		})
	}
}

func TestAnAddressKeptInAModuleIsLocatedInTheModule(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/ids.lua":     idsModule,
		"main/game.script": "local ids = require(\"main.ids\")\nuse(ids.HUD)\n",
	})
	script := loaded.Scripts["/main/game.script"]

	address, isWritten := script.AddressWrittenIn(argumentOfUse(t, script))

	if !isWritten || address.Text != "/hud#hud" || address.Location.File != "main/ids.lua" || address.Location.Position.Line != 7 {
		t.Errorf("address = %+v, written = %v; want /hud#hud at main/ids.lua:7", address, isWritten)
	}
}

// A table that lives in another file is read where it lives, where its own names
// mean what they mean. Handing it out would have its elements read in the wrong file.
func TestATableOfAnotherFileIsNotHandedOut(t *testing.T) {
	loaded := load(t, map[string]string{
		"main/ids.lua":     idsModule,
		"main/game.script": "local ids = require(\"main.ids\")\nlocal HERE = { hash(\"wall\") }\nuse(ids.SOLID)\nuse2(HERE)\n",
	})
	script := loaded.Scripts["/main/game.script"]

	if _, isATable := script.Written(argumentOfUse(t, script)).(*lua.TableLiteral); isATable {
		t.Error("the table of the module was handed out to the script that requires it")
	}
}

// The cursor is where a field is being typed, so the script around it rarely parses:
// "message." alone is not a statement and is dropped from the tree. The messages are
// read from the clauses that enclose the cursor, not from a statement that holds it.
func TestTheMessagesHandledWhereTheCursorIs(t *testing.T) {
	const cursor = "|"
	cases := map[string]struct {
		body string
		want []string
	}{
		"inside the branch of one message": {
			body: "if message_id == hash(\"hit\") then\n\tprint(message.|)\nend",
			want: []string{"hit"},
		},
		"a field half typed, alone on its line": {
			body: "if message_id == hash(\"hit\") then\n\tmessage.dam|\nend",
			want: []string{"hit"},
		},
		"nothing typed after the dot, before the end": {
			body: "if message_id == hash(\"hit\") then\n\tlocal damage = message.|\nend",
			want: []string{"hit"},
		},
		"in the condition, after the test": {
			body: "if message_id == hash(\"hit\") and message.| then\nend",
			want: []string{"hit"},
		},
		"the second branch, not the first": {
			body: "if message_id == hash(\"hit\") then\n\tprint(message.damage)\nelseif message_id == hash(\"heal\") then\n\tmessage.|\nend",
			want: []string{"heal"},
		},
		"either of two messages": {
			body: "if message_id == hash(\"hit\") or message_id == hash(\"heal\") then\n\tmessage.|\nend",
			want: []string{"heal", "hit"},
		},
		"the else of a test says nothing": {
			body: "if message_id == hash(\"hit\") then\n\tprint(1)\nelse\n\tmessage.|\nend",
			want: nil,
		},
		"after a branch that has ended": {
			body: "if message_id == hash(\"hit\") then\n\tprint(message.damage)\nend\nmessage.|",
			want: nil,
		},
		"after leaving on every other message": {
			body: "if message_id ~= hash(\"hit\") then\n\treturn\nend\nmessage.|",
			want: []string{"hit"},
		},
		"before leaving on every other message": {
			body: "message.|\nif message_id ~= hash(\"hit\") then\n\treturn\nend",
			want: nil,
		},
		"a branch inside another": {
			body: "if message_id == hash(\"hit\") then\n\tif self.alive then\n\t\tmessage.|\n\tend\nend",
			want: []string{"hit"},
		},
		"a constant of the file": {
			body: "if message_id == HIT then\n\tmessage.|\nend",
			want: []string{"hit"},
		},
		"a callback inside the branch": {
			body: "if message_id == hash(\"hit\") then\n\ttimer.delay(1, false, function()\n\t\tprint(message.|)\n\tend)\nend",
			want: []string{"hit"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			marked := "local HIT = hash(\"hit\")\n\nfunction on_message(self, message_id, message, sender)\n" + testCase.body + "\nend\n\nfunction update(self, dt)\nend\n"
			offset := strings.Index(marked, cursor)
			script := scriptOf(t, strings.Replace(marked, cursor, "", 1))
			if got := script.MessagesHandledAt(offset); !slices.Equal(got, testCase.want) {
				t.Errorf("messages = %q, want %q", got, testCase.want)
			}
		})
	}

	outside := scriptOf(t, "function update(self, dt)\n\tprint(message.x)\nend\n")
	if got := outside.MessagesHandledAt(30); got != nil {
		t.Errorf("messages = %q outside on_message", got)
	}
}
