package pls

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/build"
	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

func workshopWithoutANetwork(t *testing.T) *workshop {
	t.Helper()
	return &workshop{
		cache: t.TempDir(),
		versions: func(announce func(message string)) engine.Versions {
			return engine.Versions{Announce: announce}
		},
		store: func(announce func(message string)) docs.Store {
			return docs.Store{
				Carried:  carriedDocumentation,
				Fetch:    func(string) ([]byte, error) { return nil, errors.New("no network in tests") },
				Announce: announce,
			}
		},
	}
}

func carriedVersion(t *testing.T) string {
	t.Helper()
	version, _, hasOne := carriedDocumentation()
	if !hasOne {
		t.Fatal("this build carries no documentation")
	}
	return version
}

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	for name, content := range files {
		target := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

const scriptOfTheBall = "function on_message(self, message_id, message, sender)\n    if message_id == hash(\"collision_response\") then\n    end\nend\n"

func pong(t *testing.T) string {
	t.Helper()
	return writeProject(t, map[string]string{"game.project": "[project]\ntitle = Pong\n", "main/ball.script": scriptOfTheBall})
}

func TestCommandsThatOnlyPrint(t *testing.T) {
	carried := carriedVersion(t)
	cases := map[string]struct {
		arguments []string
		wantExit  int
		want      string
	}{
		"version":         {arguments: []string{"version"}, wantExit: exitSuccess, want: "defoldpls "},
		"help":            {arguments: []string{"help"}, wantExit: exitSuccess, want: "Usage:"},
		"unknown":         {arguments: []string{"complete"}, wantExit: exitCouldNotRun, want: `unknown command "complete"`},
		"fetch nothing":   {arguments: []string{"fetch"}, wantExit: exitCouldNotRun, want: "fetch needs at least one version"},
		"fetch too old":   {arguments: []string{"fetch", "1.5.0"}, wantExit: exitCouldNotRun, want: "the oldest version defold-vet supports is 1.6.0"},
		"fetch carried":   {arguments: []string{"fetch", carried}, wantExit: exitSuccess, want: "Defold " + carried + ": the documentation of"},
		"fetch offline":   {arguments: []string{"fetch", "1.9.0"}, wantExit: exitCouldNotRun, want: "no network in tests"},
		"too many places": {arguments: []string{"annotations", "a", "b"}, wantExit: exitCouldNotRun, want: "annotations takes one directory"},
		"a flag unknown":  {arguments: []string{"docs", "--port", "8080"}, wantExit: exitCouldNotRun, want: "flag provided but not defined"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			exitCode := run(testCase.arguments, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

			if exitCode != testCase.wantExit || !strings.Contains(stdout.String()+stderr.String(), testCase.want) {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q; want %d and %q", exitCode, stdout.String(), stderr.String(), testCase.wantExit, testCase.want)
			}
		})
	}
}

func TestAnnotationsAreWrittenForTheVersionOfTheProject(t *testing.T) {
	shop := workshopWithoutANetwork(t)
	var stdout, stderr bytes.Buffer

	exitCode := run([]string{"annotations", pong(t)}, strings.NewReader(""), &stdout, &stderr, shop)

	directory := strings.TrimSpace(stdout.String())
	if exitCode != exitSuccess || directory != filepath.Join(shop.cache, "annotations", carriedVersion(t)) {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	written, err := os.ReadFile(filepath.Join(directory, "msg.lua"))
	if err != nil || !strings.Contains(string(written), "function msg.post(receiver, message_id, message) end") {
		t.Errorf("msg.lua = %v, %.200q", err, written)
	}
	if strings.Contains(string(written), "Offline reference") {
		t.Error("the annotations link to an offline reference that nothing is serving")
	}
}

func TestAnnotationsGoWhereTheyAreAskedTo(t *testing.T) {
	output := filepath.Join(t.TempDir(), "api")
	stale := filepath.Join(output, "left_over.lua")
	if err := os.MkdirAll(output, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("-- from an older version"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	exitCode := run([]string{"annotations", "--output", output, "--defold", carriedVersion(t), pong(t)}, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

	if exitCode != exitSuccess || strings.TrimSpace(stdout.String()) != output {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(output, "go.lua")); err != nil {
		t.Errorf("go.lua was not written: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a file of an older generation was left beside the new ones")
	}
}

func TestSettingsThatCannotBeReadStopTheCommand(t *testing.T) {
	directory := writeProject(t, map[string]string{"game.project": "", "defold-vet.toml": "defold = \"1.5.0\"\n"})
	var stdout, stderr bytes.Buffer

	exitCode := run([]string{"annotations", directory}, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

	if exitCode != exitCouldNotRun || !strings.Contains(stderr.String(), "the oldest version defold-vet supports is 1.6.0") {
		t.Errorf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
}

type editor struct {
	t      *testing.T
	toIt   io.Writer
	fromIt *bufio.Reader
}

func (e *editor) send(method string, id int, params any) {
	e.t.Helper()
	sent := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		sent["id"] = id
	}
	body, err := json.Marshal(sent)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := fmt.Fprintf(e.toIt, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		e.t.Fatal(err)
	}
}

func (e *editor) receive() map[string]json.RawMessage {
	e.t.Helper()
	length := 0
	for {
		line, err := e.fromIt.ReadString('\n')
		if err != nil {
			e.t.Fatalf("the server stopped answering: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if value, isLength := strings.CutPrefix(line, "Content-Length:"); isLength {
			length, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(e.fromIt, body); err != nil {
		e.t.Fatal(err)
	}
	var received map[string]json.RawMessage
	if err := json.Unmarshal(body, &received); err != nil {
		e.t.Fatal(err)
	}
	return received
}

func (e *editor) hoverOn(document string, line, character int) string {
	e.t.Helper()
	const firstID, attempts = 100, 50
	for attempt := range attempts {
		e.send("textDocument/hover", firstID+attempt, map[string]any{
			"textDocument": map[string]string{"uri": document},
			"position":     map[string]int{"line": line, "character": character},
		})
		for {
			received := e.receive()
			if string(received["id"]) != strconv.Itoa(firstID+attempt) {
				continue
			}
			var answered struct {
				Contents struct {
					Value string `json:"value"`
				} `json:"contents"`
			}
			if json.Unmarshal(received["result"], &answered) == nil && answered.Contents.Value != "" {
				return answered.Contents.Value
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatal("the server never said anything about the name")
	return ""
}

// When an editor opens a project, the annotations of its Defold version are put
// in the cache and the reference starts, and the editor is told where both are.
func TestTheEditorIsToldWhereTheAnnotationsAndTheReferenceAre(t *testing.T) {
	shop := workshopWithoutANetwork(t)
	root := pong(t)
	toServer, fromEditor := io.Pipe()
	toEditor, fromServer := io.Pipe()
	served := make(chan int, 1)
	go func() {
		served <- run(nil, toServer, fromServer, io.Discard, shop)
		fromServer.Close()
	}()
	opened := &editor{t: t, toIt: fromEditor, fromIt: bufio.NewReader(toEditor)}
	t.Cleanup(func() { fromEditor.Close() })

	var ready annotationsReady
	arrived := make(chan struct{})
	go func() {
		defer close(arrived)
		for {
			received := opened.receive()
			if string(received["method"]) == `"`+annotationsMethod+`"` {
				_ = json.Unmarshal(received["params"], &ready)
				return
			}
		}
	}()
	opened.send("initialize", 1, map[string]string{"rootUri": "file://" + filepath.ToSlash(root)})
	opened.send("initialized", 0, struct{}{})
	select {
	case <-arrived:
	case <-time.After(15 * time.Second):
		t.Fatal("the editor was never told")
	}

	if ready.Defold != carriedVersion(t) || ready.Directory != filepath.Join(shop.cache, "annotations", carriedVersion(t)) {
		t.Errorf("ready = %+v", ready)
	}
	written, err := os.ReadFile(filepath.Join(ready.Directory, "go.lua"))
	base := strings.TrimSuffix(ready.Reference, "/"+carriedVersion(t)+"/")
	if err != nil || !strings.Contains(string(written), "[Offline reference]("+base+"/"+carriedVersion(t)+"/go#go.animate)") {
		t.Errorf("go.lua does not link to the reference at %s: %v", base, err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ready.Reference+"go", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("the reference at %s does not answer: %v", ready.Reference, err)
	}
	defer response.Body.Close()
	page, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(page), `id="go.animate"`) {
		t.Errorf("the reference answers %d without go.animate", response.StatusCode)
	}

	said := opened.hoverOn("file://"+filepath.ToSlash(root)+"/main/ball.script", 1, 30)
	if !strings.Contains(said, "- `other_id` *hash*") || !strings.Contains(said, "[Offline reference]("+base+"/"+carriedVersion(t)+"/physics#message-collision_response)") {
		t.Errorf("the hover of an engine message does not carry its documentation and the reference at %s:\n%s", base, said)
	}

	go func() { _, _ = io.Copy(io.Discard, opened.fromIt) }()
	opened.send("shutdown", 2, nil)
	opened.send("exit", 0, nil)
	select {
	case exitCode := <-served:
		if exitCode != exitSuccess {
			t.Errorf("exit code = %d", exitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not leave on exit")
	}
}

func projectOnDisk(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func asked(t *testing.T, arguments ...string) (exitCode int, stdout, stderr string) {
	t.Helper()
	var out, errors bytes.Buffer
	exitCode = run(arguments, strings.NewReader(""), &out, &errors, workshopWithoutANetwork(t))
	return exitCode, out.String(), errors.String()
}

// These two commands write into a project. They do it only when asked, they say what
// they wrote, and they never write over what is there.
func TestFilesAreMadeWhereTheyAreAskedForAndNeverOverWhatIsThere(t *testing.T) {
	root := projectOnDisk(t, map[string]string{"game.project": "[project]\ntitle = Pong\n", "main/images/a.png": "", "main/images/deep/b.png": "", "main/notes.txt": ""})

	exitCode, said, _ := asked(t, "new", "--script", filepath.Join(root, "main", "enemy.go"))
	if exitCode != exitSuccess || said != "main/enemy.go\nmain/enemy.script\n" {
		t.Fatalf("exit code = %d, said %q", exitCode, said)
	}
	gameObject, _ := os.ReadFile(filepath.Join(root, "main", "enemy.go"))
	if string(gameObject) != "components {\n  id: \"enemy\"\n  component: \"/main/enemy.script\"\n}\n" {
		t.Errorf("the game object holds %q", gameObject)
	}

	if err := os.WriteFile(filepath.Join(root, "main", "enemy.script"), []byte("-- my work\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "main", "enemy.go")); err != nil {
		t.Fatal(err)
	}
	exitCode, _, complaint := asked(t, "new", "--script", filepath.Join(root, "main", "enemy.go"))
	kept, _ := os.ReadFile(filepath.Join(root, "main", "enemy.script"))
	if _, err := os.Stat(filepath.Join(root, "main", "enemy.go")); exitCode != exitCouldNotRun || !strings.Contains(complaint, "main/enemy.script is there already, and nothing was made") || string(kept) != "-- my work\n" || err == nil {
		t.Errorf("exit code = %d, said %q, the script holds %q: want it refused with nothing written", exitCode, complaint, kept)
	}

	exitCode, said, _ = asked(t, "new", "--from", filepath.Join(root, "main", "images"), filepath.Join(root, "main", "pictures.atlas"))
	atlas, _ := os.ReadFile(filepath.Join(root, "main", "pictures.atlas"))
	if exitCode != exitSuccess || said != "main/pictures.atlas\n" || string(atlas) != "images {\n  image: \"/main/images/a.png\"\n}\nimages {\n  image: \"/main/images/deep/b.png\"\n}\n" {
		t.Errorf("exit code = %d, said %q, the atlas holds %q", exitCode, said, atlas)
	}

	if exitCode, _, complaint := asked(t, "new", filepath.Join(t.TempDir(), "enemy.go")); exitCode != exitCouldNotRun || !strings.Contains(complaint, "is in no Defold project") {
		t.Errorf("exit code = %d, said %q: want a file outside any project refused", exitCode, complaint)
	}
}

func TestAnAtlasIsBroughtInLineWithItsDirectory(t *testing.T) {
	before := "images {\n  image: \"/main/images/a.png\"\n}\nimages {\n  image: \"/main/images/gone.png\"\n}\nimages {\n  image: \"/builtins/graphics/particle_blob.png\"\n}\nmargin: 2\n"
	root := projectOnDisk(t, map[string]string{
		"game.project": "[project]\ntitle = Pong\n", "main/pictures.atlas": before,
		"main/images/a.png": "", "main/images/new.png": "", "main/images/deep/ignored.png": "", "main/images/readme.txt": "",
	})
	file := filepath.Join(root, "main", "pictures.atlas")

	exitCode, said, _ := asked(t, "atlas", file)

	after, _ := os.ReadFile(file)
	want := "images {\n  image: \"/main/images/a.png\"\n}\nimages {\n  image: \"/builtins/graphics/particle_blob.png\"\n}\nimages {\n  image: \"/main/images/new.png\"\n}\nmargin: 2\n"
	if exitCode != exitSuccess || said != "- /main/images/gone.png\n+ /main/images/new.png\n" || string(after) != want {
		t.Errorf("exit code = %d, said %q, the atlas holds:\n%s\nwant:\n%s", exitCode, said, after, want)
	}
	if exitCode, said, _ := asked(t, "atlas", file); exitCode != exitSuccess || said != "" {
		t.Errorf("a second time says %q", said)
	}
	if exitCode, said, _ := asked(t, "atlas", "--from", filepath.Join(root, "main", "images"), file); exitCode != exitSuccess || said != "+ /main/images/deep/ignored.png\n" {
		t.Errorf("looking below the directory says %q", said)
	}
}

// An editor with a shell script where its Java is, which writes down what it
// was asked instead of running Bob.
func editorThatWritesDownWhatItIsAsked(t *testing.T) (home, asked string) {
	t.Helper()
	return editorWhoseJavaAlsoRuns(t, "")
}

func editorWhoseJavaAlsoRuns(t *testing.T, more string) (home, asked string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the Java that stands in for the editor's is a shell script")
	}
	home = t.TempDir()
	asked = filepath.Join(home, "asked")
	java := filepath.Join(home, "packages", "jdk", "bin", "java")
	files := map[string]string{
		"config": "[build]\nversion = 1.13.2\neditor_sha1 = abc\n\n[launcher]\njava = ${bootstrap.resourcespath}/packages/jdk/bin/java\njar = ${bootstrap.resourcespath}/packages/defold.jar\nvmargs = -Dfile.encoding=UTF-8\n",
		java:     "#!/bin/sh\necho \"$@\" >> " + asked + "\n" + more,
	}
	for name, content := range files {
		target := name
		if !filepath.IsAbs(target) {
			target = filepath.Join(home, name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return home, asked
}

func engineThatSays(t *testing.T, file, said string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in for the engine is a shell script")
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("#!/bin/sh\necho \""+said+"\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func projectWithAnEngineOfItsOwn(t *testing.T) string {
	t.Helper()
	platform, isKnown := build.PlatformOfThisMachine()
	if !isKnown {
		t.Skip("Defold has no engine for this machine")
	}
	root := pong(t)
	engineThatSays(t, filepath.Join(root, "build", platform, "dmengine"), "the engine of the project")
	return root
}

// A project with native extensions cannot run on the engine the editor carries:
// what it calls is not in there. Defold's build tool has the engine built and
// leaves it in the project, and removes it when the project has no extension
// any more. With Bob 1.14.0 and a small extension, the engine found this way
// answered a call that the editor's own engine does not know.
func TestTheGameRunsOnTheEngineBuiltForItsExtensions(t *testing.T) {
	root := projectWithAnEngineOfItsOwn(t)
	noEditor := filepath.Join(t.TempDir(), "nothing here")

	exitCode, stdout, stderr := asked(t, "run", "--built", "--defold-home", noEditor, root)

	if exitCode != exitSuccess || stdout != "the engine of the project\n" || !strings.Contains(stderr, "the engine built for the native extensions of this project") {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want the engine in the project run, said, and no editor needed", exitCode, stdout, stderr)
	}
}

func TestAnEngineThatIsNamedIsTheOneThatRuns(t *testing.T) {
	root := projectWithAnEngineOfItsOwn(t)
	named := filepath.Join(t.TempDir(), "dmengine")
	engineThatSays(t, named, "the engine that was named")

	exitCode, stdout, stderr := asked(t, "run", "--built", "--engine", named, root)

	if exitCode != exitSuccess || stdout != "the engine that was named\n" || stderr != "" {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want the named engine and nothing else", exitCode, stdout, stderr)
	}
}

func TestAProjectWithoutAnEngineRunsOnTheOneOfTheEditor(t *testing.T) {
	shop := workshopWithoutANetwork(t)
	noEditor := errors.New("no editor")
	var announced []string
	asksTheEditor := 0

	engine, err := shop.engineFor(pong(t), "", func(message string) { announced = append(announced, message) }, func() (build.Installation, error) {
		asksTheEditor++
		return build.Installation{}, noEditor
	})

	if engine != "" || !errors.Is(err, noEditor) || asksTheEditor != 1 || len(announced) != 0 {
		t.Errorf("engine = %q, error = %v, the editor was asked %d times, announced %q; want the editor asked once and nothing said", engine, err, asksTheEditor, announced)
	}
}

// Cleaning and fetching are asked of Bob alone: neither builds the game, which
// is what would undo a clean.
func TestEachCommandAsksBobForItsOwnStep(t *testing.T) {
	cases := map[string]string{"build": "build", "resolve": "resolve", "clean": "distclean"}
	for command, step := range cases {
		t.Run(command, func(t *testing.T) {
			home, asked := editorThatWritesDownWhatItIsAsked(t)
			root := pong(t)
			var stdout, stderr bytes.Buffer

			exitCode := run([]string{command, "--defold-home", home, root}, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

			written, _ := os.ReadFile(asked)
			lines := strings.Split(strings.TrimSpace(string(written)), "\n")
			if exitCode != exitSuccess || len(lines) != 1 || !strings.HasSuffix(lines[0], "--root "+root+" --variant debug "+step) {
				t.Errorf("exit code = %d, stderr = %q, Bob was asked %q; want it asked once, for %q", exitCode, stderr.String(), lines, step)
			}
		})
	}
}

// The command an editor asks for a picture of an atlas: the build tool packs it
// away from the game, and each line of the atlas comes back with its place.
func TestAnAtlasIsPackedAndEachImageSaidWhereItLanded(t *testing.T) {
	fixture, err := filepath.Abs(filepath.Join("..", "textureset", "testdata", "props.a.texturesetc"))
	if err != nil {
		t.Fatal(err)
	}
	home, _ := editorWhoseJavaAlsoRuns(t, "mkdir -p build/default/main && cp "+fixture+" build/default/main/props.a.texturesetc\n")
	content, err := os.ReadFile(filepath.Join("..", "textureset", "testdata", "props.atlas"))
	if err != nil {
		t.Fatal(err)
	}
	root := writeProject(t, map[string]string{
		"game.project": "[project]\ntitle = Pong\n", "main/props.atlas": string(content),
		"main/images/house.png": "", "main/images/tree.png": "", "main/images/coin.png": "", "main/images/sign.png": "",
		"main/frames/walk_1.png": "", "main/frames/walk_2.png": "",
	})
	var stdout, stderr bytes.Buffer

	exitCode := run([]string{"pack", "--defold-home", home, filepath.Join(root, "main", "props.atlas")}, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

	var said packedAtlas
	if err := json.Unmarshal(stdout.Bytes(), &said); exitCode != exitSuccess || err != nil {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q, %v", exitCode, stdout.String(), stderr.String(), err)
	}
	if said.Atlas != "/main/props.atlas" || said.Root != root || said.Width != 128 || said.Height != 128 || said.Pages != 1 || len(said.Images) != 6 {
		t.Errorf("said = %+v", said)
	}
	house := packedImage{Image: "/main/images/house.png", Name: "house", Line: 2, X: 4, Y: 40, Width: 48, Height: 64, IsRotated: true}
	lastOfTheWalk := packedImage{Image: "/main/frames/walk_2.png", Name: "walk_2", Animation: "walk", Line: 19, X: 96, Y: 70, Width: 24, Height: 24}
	if said.Images[0] != house || said.Images[5] != lastOfTheWalk {
		t.Errorf("first = %+v, last = %+v", said.Images[0], said.Images[5])
	}
}

func TestPackingNeedsAnAtlasOfAProject(t *testing.T) {
	root := pong(t)
	cases := map[string]struct {
		arguments []string
		want      string
	}{
		"no file":       {arguments: []string{"pack"}, want: "pack takes one atlas"},
		"not an atlas":  {arguments: []string{"pack", filepath.Join(root, "main", "ball.script")}, want: "main/ball.script is not an atlas"},
		"not a project": {arguments: []string{"pack", filepath.Join(t.TempDir(), "props.atlas")}, want: "is in no Defold project"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			exitCode := run(testCase.arguments, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

			if exitCode != exitCouldNotRun || !strings.Contains(stderr.String(), testCase.want) {
				t.Errorf("exit code = %d, stderr = %q; want %q", exitCode, stderr.String(), testCase.want)
			}
		})
	}
}

// A bundle is what is handed out: the game built for one platform and packed.
// It is built apart from the build that runs and is debugged, since a release
// for another platform would otherwise replace what the next run needs. What
// comes after the two dashes goes to Defold's build tool as it is.
func TestBundlingAsksBobForAPlatformAVariantAndWhereTo(t *testing.T) {
	cases := map[string]struct {
		arguments []string
		forBob    []string
		want      string
		said      string
	}{
		"as it comes": {
			arguments: []string{"--platform", "wasm-web"},
			want:      "--variant release --platform wasm-web --archive --output build/bundling/wasm-web --bundle-output " + filepath.FromSlash("dist/wasm-web") + " resolve build bundle",
			said:      "bundled for wasm-web into " + filepath.FromSlash("dist/wasm-web"),
		},
		"said in full": {
			arguments: []string{"--platform", "arm64-android", "--variant", "debug", "--output", "out"},
			forBob:    []string{"--", "--bundle-format", "aab"},
			want:      "--variant debug --platform arm64-android --archive --output build/bundling/arm64-android --bundle-output out --bundle-format aab resolve build bundle",
			said:      "bundled for arm64-android into out",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			home, asked := editorThatWritesDownWhatItIsAsked(t)
			root := pong(t)
			var stdout, stderr bytes.Buffer

			arguments := slices.Concat([]string{"bundle", "--defold-home", home}, testCase.arguments, []string{root}, testCase.forBob)
			exitCode := run(arguments, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

			written, _ := os.ReadFile(asked)
			if exitCode != exitSuccess || !strings.HasSuffix(strings.TrimSpace(string(written)), "--root "+root+" "+testCase.want) || !strings.Contains(stdout.String(), testCase.said) {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q, Bob was asked %q; want it to end with %q", exitCode, stdout.String(), stderr.String(), written, testCase.want)
			}
		})
	}
}
