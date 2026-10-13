package lsp

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/insight"
)

const collectionWithOneBall = `name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
}
`

const ballWithASprite = `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: ""
}
`

const scriptThatMisspellsTheSprite = `function init(self)
    msg.post("#sprit", "disable")
end
`

type editor struct {
	t          *testing.T
	shown      []showMessageParams
	connection *connection
	root       string
	served     chan error
	nextID     int
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
	return filepath.ToSlash(directory)
}

func pongWithABrokenAddress(t *testing.T) string {
	t.Helper()
	return writeProject(t, map[string]string{
		"game.project":         "[project]\ntitle = Pong\n",
		"main/main.collection": collectionWithOneBall,
		"main/ball.go":         ballWithASprite,
		"main/ball.script":     scriptThatMisspellsTheSprite,
	})
}

func openEditorOn(t *testing.T, root string) *editor {
	t.Helper()
	return openEditorServedBy(t, root, &Server{Name: "defold-vet"})
}

func openEditorOnAServerOfNames(t *testing.T, root string) *editor {
	t.Helper()
	return openEditorServedBy(t, root, &Server{Name: "defoldpls", AnswersAboutNames: true})
}

func openEditorServedBy(t *testing.T, root string, server *Server) *editor {
	t.Helper()
	toServer, fromEditor := io.Pipe()
	toEditor, fromServer := io.Pipe()
	*server = Server{
		Name:              server.Name,
		AnswersAboutNames: server.AnswersAboutNames,
		Ready:             server.Ready,

		Version: "test",
		Pause:   time.Millisecond,
		Versions: func(announce func(string)) engine.Versions {
			return engine.Versions{Client: http.DefaultClient, Announce: announce}
		},
	}
	opened := &editor{t: t, connection: newConnection(toEditor, fromEditor), root: root, served: make(chan error, 1)}
	go func() {
		opened.served <- server.Serve(toServer, fromServer)
		fromServer.Close()
	}()
	t.Cleanup(func() {
		fromEditor.Close()
		toEditor.Close()
	})
	return opened
}

func (e *editor) request(method string, params any) *message {
	e.t.Helper()
	e.nextID++
	id := json.RawMessage(strings.Repeat("1", e.nextID))
	encoded, err := json.Marshal(params)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.connection.write(message{ID: &id, Method: method, Params: encoded}); err != nil {
		e.t.Fatal(err)
	}
	for {
		received := e.receive()
		if received.ID != nil && string(*received.ID) == string(id) {
			return received
		}
		var shown showMessageParams
		if received.Method == "window/showMessage" && json.Unmarshal(received.Params, &shown) == nil {
			e.shown = append(e.shown, shown)
		}
	}
}

func (e *editor) notify(method string, params any) {
	e.t.Helper()
	if err := e.connection.notify(method, params); err != nil {
		e.t.Fatal(err)
	}
}

func (e *editor) receive() *message {
	e.t.Helper()
	type outcome struct {
		received *message
		err      error
	}
	arrived := make(chan outcome, 1)
	go func() {
		received, err := e.connection.read()
		arrived <- outcome{received, err}
	}()
	select {
	case got := <-arrived:
		if got.err != nil {
			e.t.Fatalf("the server stopped answering: %v", got.err)
		}
		return got.received
	case <-time.After(10 * time.Second):
		e.t.Fatal("the server did not answer in time")
		return nil
	}
}

func (e *editor) diagnosticsOf(file string) []diagnostic {
	e.t.Helper()
	for {
		received := e.receive()
		if received.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var published publishDiagnosticsParams
		if err := json.Unmarshal(received.Params, &published); err != nil {
			e.t.Fatal(err)
		}
		if published.URI == uriOf(e.root+"/"+file) {
			return published.Diagnostics
		}
	}
}

func (e *editor) start() *message {
	e.t.Helper()
	answer := e.request("initialize", initializeParams{RootURI: uriOf(e.root)})
	e.notify("initialized", struct{}{})
	return answer
}

func TestTheServerSaysWhatItDoes(t *testing.T) {
	answer := openEditorOn(t, pongWithABrokenAddress(t)).start()

	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatal(err)
	}
	var result initializeResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Capabilities.TextDocumentSync.OpenClose || result.Capabilities.TextDocumentSync.Change != syncFullText || result.ServerInfo.Name != "defold-vet" {
		t.Errorf("result = %+v, want a server that takes whole documents as they change", result)
	}
}

// defold-vet lsp sends the problems alone, and defoldpls everything else: decision 0011.
// An editor must not be told of completions that the server it started will refuse.
func TestOnlyAServerOfNamesAnswersAboutThem(t *testing.T) {
	root := pongWithABrokenAddress(t)
	document := uriOf(root + "/main/ball.script")

	ofProblems := openEditorOn(t, root)
	told := resultOf[initializeResult](t, ofProblems.start()).Capabilities
	if told.CompletionProvider != nil || told.DefinitionProvider || told.HoverProvider || told.CodeActionProvider {
		t.Errorf("a server of problems alone says it does more: %+v", told)
	}
	ofProblems.diagnosticsOf("main/ball.script")
	for _, method := range []string{"textDocument/completion", "textDocument/definition", "textDocument/hover", "textDocument/codeAction"} {
		if refused := ofProblems.request(method, at(document, 1, 12)); refused.Error == nil || refused.Error.Code != methodNotFound {
			t.Errorf("%s was answered by a server of problems alone: %+v", method, refused)
		}
	}

	ofNames := openEditorOnAServerOfNames(t, root)
	told = resultOf[initializeResult](t, ofNames.start()).Capabilities
	if told.CompletionProvider == nil || !told.DefinitionProvider || !told.HoverProvider || !told.CodeActionProvider {
		t.Errorf("a server of names does not say all it does: %+v", told)
	}
}

func TestTheProblemsOfTheProjectArriveWhenTheEditorOpensIt(t *testing.T) {
	opened := openEditorOn(t, pongWithABrokenAddress(t))
	opened.start()

	diagnostics := opened.diagnosticsOf("main/ball.script")

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one", diagnostics)
	}
	got := diagnostics[0]
	want := textRange{Start: position{Line: 1, Character: 13}, End: position{Line: 1, Character: 21}}
	if got.Range != want || got.Code != "unresolved-url" || got.Source != "defold-vet" || got.Severity != severityError {
		t.Errorf("diagnostic = %+v, want unresolved-url over the quoted address", got)
	}
	if !strings.Contains(got.Message, `did you mean "sprite"?`) {
		t.Errorf("message = %q", got.Message)
	}
}

// What is typed and not yet saved is what gets checked, and a problem that is
// fixed has to be taken back.
func TestProblemsFollowWhatIsTypedBeforeItIsSaved(t *testing.T) {
	opened := openEditorOn(t, pongWithABrokenAddress(t))
	opened.start()
	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 1 {
		t.Fatalf("diagnostics at the start = %+v, want one", got)
	}
	document := uriOf(opened.root + "/main/ball.script")

	fixed := strings.Replace(scriptThatMisspellsTheSprite, "#sprit", "#sprite", 1)
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: document, Text: fixed}})
	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 0 {
		t.Errorf("diagnostics after the fix = %+v, want none", got)
	}

	brokenAgain := strings.Replace(fixed, "#sprite", "#spriet", 1)
	opened.notify("textDocument/didChange", didChangeParams{TextDocument: textDocument{URI: document}, ContentChanges: []contentChange{{Text: brokenAgain}}})
	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 1 || !strings.Contains(got[0].Message, "spriet") {
		t.Errorf("diagnostics after breaking it again = %+v, want the new mistake", got)
	}

	opened.notify("textDocument/didClose", didCloseParams{TextDocument: textDocument{URI: document}})
	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 1 || !strings.Contains(got[0].Message, `"#sprit"`) {
		t.Errorf("diagnostics after closing without saving = %+v, want what is on disk", got)
	}
}

// Between two checks one file changed, the one that is being typed in. The
// server keeps what it read from the others, and reads that one again.
func TestTheServerReadsAgainOnlyTheFileThatWasTyped(t *testing.T) {
	server := &Server{Name: "defold-vet"}
	opened := openEditorServedBy(t, pongWithABrokenAddress(t), server)
	opened.start()
	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 1 {
		t.Fatalf("diagnostics at the start = %+v, want one", got)
	}

	fixed := strings.Replace(scriptThatMisspellsTheSprite, "#sprit", "#sprite", 1)
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: uriOf(opened.root + "/main/ball.script"), Text: fixed}})
	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 0 {
		t.Fatalf("diagnostics after the fix = %+v, want none", got)
	}

	server.checking.Lock()
	read, kept := server.readBefore.ReadAnew, server.readBefore.Reused
	server.checking.Unlock()
	const filesOfTheProject = 3
	if read != filesOfTheProject+1 || kept < filesOfTheProject-1 || (read+kept)%filesOfTheProject != 0 {
		t.Errorf("the server read %d files and kept %d from an earlier check; want the three of the project read once, and one of them once more", read, kept)
	}
}

func TestAChangeOnDiskIsSeenWhenTheEditorSaysSo(t *testing.T) {
	opened := openEditorOn(t, pongWithABrokenAddress(t))
	opened.start()
	opened.diagnosticsOf("main/ball.script")

	fixed := strings.Replace(scriptThatMisspellsTheSprite, "#sprit", "#sprite", 1)
	if err := os.WriteFile(filepath.FromSlash(opened.root+"/main/ball.script"), []byte(fixed), 0o600); err != nil {
		t.Fatal(err)
	}
	opened.notify("workspace/didChangeWatchedFiles", struct{}{})

	if got := opened.diagnosticsOf("main/ball.script"); len(got) != 0 {
		t.Errorf("diagnostics = %+v, want none", got)
	}
}

func TestWhatCannotBeVerifiedIsAWarning(t *testing.T) {
	root := writeProject(t, map[string]string{
		"game.project":       "[project]\ntitle = Pong\n",
		"main/paddle.script": "function update(self)\n    go.set(\".\", self.property, 1)\nend\n",
	})
	opened := openEditorOn(t, root)
	opened.start()

	diagnostics := opened.diagnosticsOf("main/paddle.script")

	var codes []string
	for _, found := range diagnostics {
		codes = append(codes, found.Code)
		if found.Code == "unverifiable" && found.Severity != severityWarning {
			t.Errorf("severity of %s = %d, want a warning", found.Code, found.Severity)
		}
	}
	if !strings.Contains(strings.Join(codes, " "), "unverifiable") {
		t.Errorf("codes = %v, want unverifiable among them", codes)
	}
}

// A file nothing uses is not a mistake in the game: it may be a module about to
// be required, or what a tool has just generated. A project opened for the
// first time must not be painted red for it.
func TestAFileNothingUsesIsAWarning(t *testing.T) {
	root := writeProject(t, map[string]string{
		"game.project": "[project]\ntitle = Pong\n",
		"ids.lua":      "local M = {}\nM.BALL = hash(\"ball\")\nreturn M\n",
	})
	opened := openEditorOn(t, root)
	opened.start()

	diagnostics := opened.diagnosticsOf("ids.lua")

	if len(diagnostics) != 1 || diagnostics[0].Code != "unused-file" {
		t.Fatalf("diagnostics = %+v, want the file reported as unused", diagnostics)
	}
	if diagnostics[0].Severity != severityWarning {
		t.Errorf("severity = %d, want a warning", diagnostics[0].Severity)
	}
}

func TestSettingsThatCannotBeReadAreShownToTheUser(t *testing.T) {
	root := pongWithABrokenAddress(t)
	if err := os.WriteFile(filepath.FromSlash(root+"/defold-vet.toml"), []byte("[checks]\nunresolved-ulr = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened := openEditorOn(t, root)
	opened.start()

	for {
		received := opened.receive()
		if received.Method != "window/showMessage" {
			continue
		}
		if !strings.Contains(string(received.Params), "is not a check") {
			t.Errorf("message = %s, want it to say what is wrong with the settings", received.Params)
		}
		return
	}
}

func TestTheServerAnswersWhatItDoesNotDoAndLeavesWhenTold(t *testing.T) {
	opened := openEditorOn(t, pongWithABrokenAddress(t))

	if early := opened.request("textDocument/rename", struct{}{}); early.Error == nil || early.Error.Code != serverNotStarted {
		t.Errorf("answer before initialize = %+v, want a refusal", early)
	}
	opened.start()
	if unknown := opened.request("textDocument/rename", struct{}{}); unknown.Error == nil || unknown.Error.Code != methodNotFound {
		t.Errorf("answer to rename = %+v, want it refused as not found", unknown)
	}
	if answer := opened.request("shutdown", nil); answer.Error != nil {
		t.Errorf("answer to shutdown = %+v", answer)
	}
	opened.notify("exit", nil)

	select {
	case err := <-opened.served:
		if err != nil {
			t.Errorf("Serve returned %v, want it to end cleanly", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not leave on exit")
	}
}

func TestADirectoryThatIsNotAProjectGetsNoDiagnostics(t *testing.T) {
	opened := openEditorOn(t, writeProject(t, map[string]string{"notes.txt": "nothing here"}))
	opened.start()

	if answer := opened.request("shutdown", nil); answer.Error != nil {
		t.Errorf("answer to shutdown = %+v", answer)
	}
}

func resultOf[T any](t *testing.T, answer *message) T {
	t.Helper()
	var result T
	if answer.Error != nil {
		t.Fatalf("the request was refused: %+v", answer.Error)
	}
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func openOnPong(t *testing.T) (opened *editor, document string) {
	t.Helper()
	opened = openEditorOnAServerOfNames(t, pongWithABrokenAddress(t))
	opened.start()
	opened.diagnosticsOf("main/ball.script")
	return opened, uriOf(opened.root + "/main/ball.script")
}

func at(document string, line, character int) positionParams {
	return positionParams{TextDocument: textDocument{URI: document}, Position: position{Line: line, Character: character}}
}

// The names are offered for what is typed, saved or not, and each replaces the
// part of the string it stands for.
func TestNamesAreOfferedWhileTyping(t *testing.T) {
	opened, document := openOnPong(t)
	typed := "function init(self)\n    msg.post(\"#\n    msg.post(\"#sprite\", \"dis\")\nend\n"
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: document, Text: typed}})

	components := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 1, 15)))

	labels := map[string]completionItem{}
	for _, item := range components {
		labels[item.Label] = item
	}
	sprite, isOffered := labels["sprite"]
	if !isOffered || labels["ball"].Label == "" {
		t.Fatalf("offered %+v, want the components of the ball", components)
	}
	want := textRange{Start: position{Line: 1, Character: 15}, End: position{Line: 1, Character: 15}}
	if sprite.TextEdit.Range != want || sprite.TextEdit.NewText != "sprite" || sprite.Kind != kindOfAReference || !strings.Contains(sprite.Detail, "sprite") {
		t.Errorf("sprite = %+v, want it to be put after the #", sprite)
	}

	messages := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 2, 27)))
	var disable *completionItem
	for index := range messages {
		if messages[index].Label == "disable" {
			disable = &messages[index]
		}
	}
	replaced := textRange{Start: position{Line: 2, Character: 25}, End: position{Line: 2, Character: 28}}
	if disable == nil || disable.TextEdit.Range != replaced || disable.Kind != kindOfAnEvent {
		t.Errorf("disable = %+v among %d, want it to replace what was typed of the message", disable, len(messages))
	}

	if nowhere := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 0, 3))); len(nowhere) != 0 {
		t.Errorf("offered %+v where no name goes", nowhere)
	}
}

const scriptThatReadsACollision = "function on_message(self, message_id, message, sender)\n    if message_id == hash(\"collision_response\") then\n        print(message.)\n    end\nend\n"

func otherIDAmong(items []completionItem) *completionItem {
	for index := range items {
		if items[index].Label == "other_id" {
			return &items[index]
		}
	}
	return nil
}

func TestTheFieldsOfAMessageAreOfferedAfterTheDot(t *testing.T) {
	opened, document := openOnPong(t)
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: document, Text: scriptThatReadsACollision}})

	told := resultOf[initializeResult](t, openEditorOnAServerOfNames(t, opened.root).start()).Capabilities
	if !slices.Contains(told.CompletionProvider.TriggerCharacters, ".") {
		t.Errorf("the dot does not ask for names: %q", told.CompletionProvider.TriggerCharacters)
	}

	otherID := otherIDAmong(resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 2, 22))))

	afterTheDot := textRange{Start: position{Line: 2, Character: 22}, End: position{Line: 2, Character: 22}}
	if otherID == nil || otherID.TextEdit.Range != afterTheDot || otherID.Kind != kindOfAField || otherID.Detail != "field of collision_response" {
		t.Fatalf("other_id = %+v, want it to be put after the dot", otherID)
	}
	if otherID.Documentation == nil || otherID.Documentation.Kind != markdownKind || !strings.Contains(otherID.Documentation.Value, "`other_id`") {
		t.Errorf("other_id is offered with %+v", otherID.Documentation)
	}
	if afterAnotherDot := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 1, 7))); len(afterAnotherDot) != 0 {
		t.Errorf("offered %+v where no name goes", afterAnotherDot)
	}
}

// The server does not load documentation: the program that starts it hands it over,
// once it has it, and from then on the answers carry what it says.
func TestTheDocumentationHandedToASessionIsUsedInTheAnswers(t *testing.T) {
	newest, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	handed := make(chan struct{})
	server := &Server{Name: "defoldpls", AnswersAboutNames: true, Ready: func(session Session) {
		session.Consult(insight.Documentation{
			Links: docs.Links{OfflineReference: "http://127.0.0.1:45565"},
			Reference: &docs.Reference{Version: newest.Defold, Modules: []docs.Module{{
				Namespace: "physics", Name: "Collision object",
				Messages: []docs.Message{{
					Name: "collision_response", Description: "Reports a collision.",
					Fields: []docs.Value{{Name: "other_id", Types: []string{"hash"}, Doc: "who was hit"}},
				}},
			}}},
		})
		close(handed)
	}}
	opened := openEditorServedBy(t, pongWithABrokenAddress(t), server)
	opened.start()
	opened.diagnosticsOf("main/ball.script")
	<-handed
	document := uriOf(opened.root + "/main/ball.script")
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: document, Text: scriptThatReadsACollision}})

	otherID := otherIDAmong(resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 2, 22))))
	if otherID == nil || otherID.Detail != "hash · field of collision_response" || !strings.Contains(otherID.Documentation.Value, "`other_id` *hash* — who was hit") {
		t.Errorf("other_id = %+v, want its type and what the documentation says", otherID)
	}

	said := resultOf[hover](t, opened.request("textDocument/hover", at(document, 1, 35))).Contents.Value
	link := "[Offline reference](http://127.0.0.1:45565/" + newest.Defold + "/physics#message-collision_response) · [defold.com](https://defold.com/ref/physics-lua#collision_response)"
	if !strings.Contains(said, "Reports a collision.") || !strings.Contains(said, link) {
		t.Errorf("the hover lacks the documentation or its links:\n%s", said)
	}
}

func TestANameLeadsToWhereItIsDeclared(t *testing.T) {
	opened, document := openOnPong(t)
	fixed := strings.Replace(scriptThatMisspellsTheSprite, "#sprit", "#sprite", 1)
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: document, Text: fixed}})
	opened.diagnosticsOf("main/ball.script")

	found := resultOf[[]location](t, opened.request("textDocument/definition", at(document, 1, 17)))

	want := location{URI: uriOf(opened.root + "/main/ball.go"), Range: textRange{Start: position{Line: 5, Character: 6}, End: position{Line: 5, Character: 14}}}
	if len(found) != 1 || found[0] != want {
		t.Errorf("found %+v, want the id of the sprite in the game object, %+v", found, want)
	}
	if nothing := resultOf[[]location](t, opened.request("textDocument/definition", at(document, 0, 3))); len(nothing) != 0 {
		t.Errorf("found %+v where no name is", nothing)
	}
}

func TestANameIsDescribedWhereTheCursorRests(t *testing.T) {
	opened, document := openOnPong(t)

	said := resultOf[hover](t, opened.request("textDocument/hover", at(document, 1, 16)))

	if said.Contents.Kind != markdownKind || !strings.Contains(said.Contents.Value, "`/ball` has no component `sprit`") {
		t.Errorf("said %+v, want what is wrong with the address", said)
	}
	if want := (textRange{Start: position{Line: 1, Character: 14}, End: position{Line: 1, Character: 20}}); said.Range != want {
		t.Errorf("range = %+v, want the inside of the string, %+v", said.Range, want)
	}
	if answer := opened.request("textDocument/hover", at(document, 0, 3)); answer.Error != nil || answer.Result != nil {
		t.Errorf("answer = %+v, want nothing said where no name is", answer)
	}
}

func TestAProblemThatSaysWhatWasMeantCanBeFixed(t *testing.T) {
	opened, document := openOnPong(t)
	onTheLine := positionParams{TextDocument: textDocument{URI: document}, Range: textRange{Start: position{Line: 1, Character: 0}, End: position{Line: 1, Character: 30}}}

	actions := resultOf[[]codeAction](t, opened.request("textDocument/codeAction", onTheLine))

	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want one", actions)
	}
	action := actions[0]
	edits := action.Edit.Changes[document]
	replaced := textRange{Start: position{Line: 1, Character: 15}, End: position{Line: 1, Character: 20}}
	if action.Title != `Change to "sprite"` || action.Kind != quickFixKind || len(edits) != 1 || edits[0].Range != replaced || edits[0].NewText != "sprite" {
		t.Errorf("action = %+v, want the name of the component replaced, after the #", action)
	}
	if len(action.Diagnostics) != 1 || action.Diagnostics[0].Code != "unresolved-url" {
		t.Errorf("the action is not tied to its problem: %+v", action.Diagnostics)
	}

	elsewhere := positionParams{TextDocument: textDocument{URI: document}, Range: textRange{Start: position{Line: 0, Character: 0}, End: position{Line: 0, Character: 5}}}
	if none := resultOf[[]codeAction](t, opened.request("textDocument/codeAction", elsewhere)); len(none) != 0 {
		t.Errorf("actions = %+v on a line without problems", none)
	}
}

func TestWhatIsReplacedByAFix(t *testing.T) {
	cases := []struct {
		written, suggestion, want string
	}{
		{written: `"#bounce_sond"`, suggestion: "bounce_sound", want: `"#bounce_sound"`},
		{written: `"/game#gaem"`, suggestion: "game", want: `"/game#game"`},
		{written: `"/gaem#game"`, suggestion: "/game", want: `"/game#game"`},
		{written: `"main:/gaem"`, suggestion: "/game", want: `"main:/game"`},
		{written: `"pozition.x"`, suggestion: "position", want: `"position.x"`},
		{written: `"bal_out"`, suggestion: "ball_out", want: `"ball_out"`},
		{written: `'rol'`, suggestion: "roll", want: `'roll'`},
		{written: `gaem`, suggestion: "game", want: `game`},
		{written: `"restart/lable"`, suggestion: "restart/label", want: `"restart/label"`},
		{written: `"something_else"`, suggestion: "entirely_other", want: `"entirely_other"`},
	}
	for _, testCase := range cases {
		start, end, replacement := whatToReplace(testCase.written, testCase.suggestion)
		if got := testCase.written[:start] + replacement + testCase.written[end:]; got != testCase.want {
			t.Errorf("fixing %s towards %q gives %s, want %s", testCase.written, testCase.suggestion, got, testCase.want)
		}
	}
}

func TestPositionsOfTheEditorAndOffsetsOfTheFile(t *testing.T) {
	content := []byte("local a = 1\nação = \"x\"\nlast")
	cases := []struct {
		where  position
		offset int
	}{
		{where: position{Line: 0, Character: 0}, offset: 0},
		{where: position{Line: 0, Character: 6}, offset: 6},
		{where: position{Line: 1, Character: 0}, offset: 12},
		{where: position{Line: 1, Character: 4}, offset: 18},
		{where: position{Line: 1, Character: 8}, offset: 22},
		{where: position{Line: 2, Character: 4}, offset: len(content)},
	}
	for _, testCase := range cases {
		if got := offsetAt(content, testCase.where); got != testCase.offset {
			t.Errorf("offsetAt(%+v) = %d, want %d", testCase.where, got, testCase.offset)
		}
		if got := positionAt(content, testCase.offset); got != testCase.where {
			t.Errorf("positionAt(%d) = %+v, want %+v", testCase.offset, got, testCase.where)
		}
	}
	if got := offsetAt(content, position{Line: 9, Character: 3}); got != len(content) {
		t.Errorf("a position past the end gives %d, want the end", got)
	}
}

// A problem with a name is located where the expression that holds it starts,
// which for an id is the word hash. Underlining that word alone left the name
// itself looking fine, and pointing at the name showed nothing of the problem.
// The whole of hash("name") is underlined, and anything else as before.
func TestAProblemWithAHashedNameUnderlinesTheWholeCall(t *testing.T) {
	cases := map[string]struct {
		line   string
		column int
		want   string
	}{
		"an id that is hashed":       {line: `    if action_id == hash("p3_up") then`, column: 21, want: `hash("p3_up")`},
		"a string":                   {line: `    msg.post("#spirte", "disable")`, column: 14, want: `"#spirte"`},
		"a call of something else":   {line: `    local id = hash(name)`, column: 16, want: `hash`},
		"a call of more than a name": {line: `    go.get("#sprite", "tint")`, column: 5, want: `go.get`},
		"a call that is not closed":  {line: `    if id == hash("p3_up" then`, column: 14, want: `hash`},
		"a name alone":               {line: `    speed = fast`, column: 13, want: `fast`},
	}
	for name, testCase := range cases {
		content := []byte("first line\n" + testCase.line + "\nlast line\n")
		shown := rangeOfAProblem(content, 2, testCase.column)
		if got := testCase.line[shown.Start.Character:shown.End.Character]; shown.Start.Line != 1 || shown.End.Line != 1 || got != testCase.want {
			t.Errorf("%s: underlined %q on line %d, want %q", name, got, shown.Start.Line+1, testCase.want)
		}
	}
}

const (
	bindingOfTheJump   = "key_trigger {\n  input: KEY_SPACE\n  action: \"jump\"\n}\n"
	scriptThatJumps    = "function on_input(self, action_id, action)\n    if action_id == hash(\"jump\") then\n        msg.post(\"#ball\", \"enable\")\n    end\nend\n"
	settingsWithInputs = "[project]\ntitle = Pong\n\n[input]\ngame_binding = /input/game.input_bindingc\n"
)

func openOnAGameThatJumps(t *testing.T) (opened *editor, script, binding string) {
	t.Helper()
	opened = openEditorOnAServerOfNames(t, writeProject(t, map[string]string{
		"game.project":             settingsWithInputs,
		"input/game.input_binding": bindingOfTheJump,
		"main/main.collection":     collectionWithOneBall,
		"main/ball.go":             ballWithASprite,
		"main/ball.script":         scriptThatJumps,
	}))
	opened.start()
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: uriOf(opened.root + "/main/ball.script"), Text: scriptThatJumps}})
	answered := opened.request("textDocument/hover", at(uriOf(opened.root+"/main/ball.script"), 1, 28))
	for attempt := 0; answered.Result == nil && attempt < 100; attempt++ {
		time.Sleep(10 * time.Millisecond)
		answered = opened.request("textDocument/hover", at(uriOf(opened.root+"/main/ball.script"), 1, 28))
	}
	return opened, uriOf(opened.root + "/main/ball.script"), uriOf(opened.root + "/input/game.input_binding")
}

func TestANameIsFoundWhereverItIsWritten(t *testing.T) {
	opened, script, binding := openOnAGameThatJumps(t)

	found := resultOf[[]location](t, opened.request("textDocument/references", at(script, 1, 28)))

	want := []location{
		{URI: script, Range: textRange{Start: position{Line: 1, Character: 26}, End: position{Line: 1, Character: 30}}},
		{URI: binding, Range: textRange{Start: position{Line: 2, Character: 11}, End: position{Line: 2, Character: 15}}},
	}
	if !slices.Equal(found, want) {
		t.Errorf("found %+v, want %+v", found, want)
	}
}

// A rename changes a data file together with the scripts. The server writes nothing:
// it answers with the edits, each one checked against the text it replaces, and the
// editor applies them.
func TestARenameIsAnsweredWithEditsToScriptsAndDataFiles(t *testing.T) {
	opened, script, binding := openOnAGameThatJumps(t)

	offered := resultOf[whatCanBeRenamed](t, opened.request("textDocument/prepareRename", at(script, 1, 28)))
	if offered.Placeholder != "jump" || offered.Range.Start.Character != 26 || offered.Range.End.Character != 30 {
		t.Errorf("offered = %+v, want the action between its quotes", offered)
	}

	asked := at(script, 1, 28)
	asked.NewName = "leap"
	edits := resultOf[workspaceEdit](t, opened.request("textDocument/rename", asked)).Changes

	inTheScript := []textEdit{{Range: textRange{Start: position{Line: 1, Character: 26}, End: position{Line: 1, Character: 30}}, NewText: "leap"}}
	inTheBinding := []textEdit{{Range: textRange{Start: position{Line: 2, Character: 11}, End: position{Line: 2, Character: 15}}, NewText: "leap"}}
	if len(edits) != 2 || !slices.Equal(edits[script], inTheScript) || !slices.Equal(edits[binding], inTheBinding) {
		t.Errorf("edits = %+v", edits)
	}
	if onDisk, _ := os.ReadFile(filepath.FromSlash(opened.root + "/input/game.input_binding")); string(onDisk) != bindingOfTheJump {
		t.Errorf("the server changed the binding itself:\n%s", onDisk)
	}
}

func TestARenameThatCannotBeDoneSaysWhy(t *testing.T) {
	opened, script, _ := openOnAGameThatJumps(t)

	ofTheEngine := opened.request("textDocument/prepareRename", at(script, 2, 30))
	if ofTheEngine.Error == nil || ofTheEngine.Error.Code != requestFailed || !strings.Contains(ofTheEngine.Error.Message, "is a message of the engine") {
		t.Errorf("answer = %+v, want a message of the engine refused", ofTheEngine)
	}

	if elsewhere := opened.request("textDocument/prepareRename", at(script, 0, 12)); elsewhere.Error != nil || elsewhere.Result != nil {
		t.Errorf("answer = %+v, want nothing said where no name of the project is, so that another server can rename there", elsewhere)
	}

	broken := at(script, 1, 28)
	broken.NewName = `le"ap`
	if refused := opened.request("textDocument/rename", broken); refused.Error == nil || !strings.Contains(refused.Error.Message, "cannot name a action") {
		t.Errorf("answer = %+v, want the name refused", refused)
	}

	if err := os.WriteFile(filepath.FromSlash(opened.root+"/input/game.input_binding"), []byte("# moved\n"+bindingOfTheJump), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := at(script, 1, 28)
	stale.NewName = "leap"
	if refused := opened.request("textDocument/rename", stale); refused.Error == nil || !strings.Contains(refused.Error.Message, "input/game.input_binding changed since the project was last read, so nothing was renamed") {
		t.Errorf("answer = %+v, want the rename refused because a file moved under it", refused)
	}
}

func TestTheEditorIsWarnedOfWhatARenameLeavesAlone(t *testing.T) {
	opened, script, _ := openOnAGameThatJumps(t)
	withAProperty := "go.property(\"jump_action\", hash(\"jump\"))\n" + scriptThatJumps
	opened.notify("textDocument/didChange", didChangeParams{TextDocument: textDocument{URI: script}, ContentChanges: []contentChange{{Text: withAProperty}}})

	asked := at(script, 2, 28)
	asked.NewName = "leap"
	edits := resultOf[workspaceEdit](t, opened.request("textDocument/rename", asked)).Changes

	if len(edits[script]) != 1 || edits[script][0].Range.Start.Line != 2 {
		t.Errorf("edits = %+v, want the comparison alone changed in the script", edits[script])
	}
	want := `Renaming "jump" leaves the same text alone in main/ball.script:1, because what it is used for there cannot be read. Look at it before going on.`
	if len(opened.shown) != 1 || opened.shown[0].Message != want || opened.shown[0].Type != messageIsWarning {
		t.Errorf("shown = %+v, want one warning: %s", opened.shown, want)
	}
}

// The line of a setting Defold removed does nothing, and the way to be rid of the
// warning is to be rid of the line. The editor is handed the edit and applies it:
// the whole line and its end, or up to the end of the file when it is the last.
func TestASettingDefoldRemovedIsAWarningWithAFixThatRemovesTheLine(t *testing.T) {
	cases := map[string]struct {
		settings string
		removed  textRange
	}{
		"a line in the middle": {
			settings: "[project]\ntitle = Pong\n\n[script]\nshared_state = 1\n\n[display]\nwidth = 960\n",
			removed:  textRange{Start: position{Line: 4}, End: position{Line: 5}},
		},
		"the last line, without an end": {
			settings: "[project]\ntitle = Pong\n\n[script]\nshared_state = 1",
			removed:  textRange{Start: position{Line: 4}, End: position{Line: 4, Character: 16}},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			opened := openEditorOnAServerOfNames(t, writeProject(t, map[string]string{
				"game.project":         testCase.settings,
				"main/main.collection": collectionWithOneBall,
				"main/ball.go":         ballWithASprite,
			}))
			opened.start()
			settings := uriOf(opened.root + "/game.project")

			var removed []diagnostic
			for _, problem := range opened.diagnosticsOf("game.project") {
				if problem.Code == "removed-setting" {
					removed = append(removed, problem)
				}
			}
			if len(removed) != 1 || removed[0].Severity != severityWarning || removed[0].Range.Start.Line != 4 {
				t.Fatalf("problems = %+v, want one warning on the line of the setting", removed)
			}

			onTheKey := positionParams{TextDocument: textDocument{URI: settings}, Range: textRange{Start: position{Line: 4, Character: 3}, End: position{Line: 4, Character: 3}}}
			fixes := resultOf[[]codeAction](t, opened.request("textDocument/codeAction", onTheKey))
			if len(fixes) != 1 || fixes[0].Title != "Remove the line" || fixes[0].Kind != quickFixKind {
				t.Fatalf("fixes = %+v", fixes)
			}
			if edits := fixes[0].Edit.Changes[settings]; len(edits) != 1 || edits[0].NewText != "" || edits[0].Range != testCase.removed {
				t.Errorf("the fix edits %+v, want %+v emptied", fixes[0].Edit.Changes, testCase.removed)
			}

			said := resultOf[hover](t, opened.request("textDocument/hover", at(settings, 4, 3))).Contents.Value
			if !strings.Contains(said, "**script.shared_state**") || !strings.Contains(said, "Defold 1.14.0 removed this setting") {
				t.Errorf("the hover of the removed setting says:\n%s", said)
			}
		})
	}
}

const settingsWithATypo = "[project]\ntitle = Pong\n\n[display]\nwidht = 960\nhigh_dpi = \n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n"

// game.project is a form in the Defold editor. Here it is a text file, and the schema
// of the project's Defold version says what goes in it.
func TestTheSettingsFileIsCompletedExplainedAndFixed(t *testing.T) {
	opened := openEditorOnAServerOfNames(t, writeProject(t, map[string]string{
		"game.project":         settingsWithATypo,
		"main/main.collection": collectionWithOneBall,
		"main/ball.go":         ballWithASprite,
		"main/ball.script":     scriptThatMisspellsTheSprite,
	}))
	opened.start()
	problems := opened.diagnosticsOf("game.project")
	settings := uriOf(opened.root + "/game.project")
	if len(problems) != 1 || problems[0].Code != "unknown-setting" || problems[0].Range.Start.Line != 4 {
		t.Fatalf("problems = %+v, want the key that is almost one", problems)
	}

	labels := func(items []completionItem) []string {
		names := make([]string, 0, len(items))
		for _, item := range items {
			names = append(names, item.Label)
		}
		return names
	}
	keys := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(settings, 4, 2)))
	if names := labels(keys); !slices.Contains(names, "width") || !slices.Contains(names, "height") || slices.Contains(names, "high_dpi") || slices.Contains(names, "title") {
		t.Errorf("keys offered in [display]: %q, want those of the section that are not set yet", names)
	}
	values := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(settings, 5, 11)))
	if names := labels(values); !slices.Equal(names, []string{"0", "1"}) {
		t.Errorf("values offered for high_dpi: %q", names)
	}

	said := resultOf[hover](t, opened.request("textDocument/hover", at(settings, 5, 3))).Contents.Value
	if !strings.Contains(said, "**display.high_dpi** · bool") || !strings.Contains(said, "Default: `0`") || !strings.Contains(said, "https://defold.com/manuals/project-settings/") {
		t.Errorf("the hover of a setting says:\n%s", said)
	}

	leads := resultOf[[]location](t, opened.request("textDocument/definition", at(settings, 8, 25)))
	if len(leads) != 1 || leads[0].URI != uriOf(opened.root+"/main/main.collection") {
		t.Errorf("the collection the settings name leads to %+v", leads)
	}

	onTheKey := positionParams{TextDocument: textDocument{URI: settings}, Range: textRange{Start: position{Line: 4, Character: 0}, End: position{Line: 4, Character: 5}}}
	fixes := resultOf[[]codeAction](t, opened.request("textDocument/codeAction", onTheKey))
	if len(fixes) != 1 || fixes[0].Title != `Change to "width"` {
		t.Fatalf("fixes = %+v", fixes)
	}
	if edit := fixes[0].Edit.Changes[settings]; len(edit) != 1 || edit[0].NewText != "width" || edit[0].Range.Start.Character != 0 || edit[0].Range.End.Character != 5 {
		t.Errorf("the fix edits %+v, want the key replaced", fixes[0].Edit.Changes)
	}
}

// A game object is a list of other files, and reading one means following
// them. Going to the definition of a path, in a game object, a collection or
// any other data file, opens the file it names. The names of such a file are
// not found or renamed from inside it.
func TestAFileNamedInADataFileCanBeGoneTo(t *testing.T) {
	object := "components {\n  id: \"script\"\n  component: \"/pickups/feedback.script\"\n}\nembedded_components {\n  id: \"sprite\"\n  type: \"sprite\"\n  data: \"tile_set: \\\"/pickups/pickups.atlas\\\"\\n\"\n  \"default_animation: \\\"coin\\\"\\n\"\n  \"\"\n}\n"
	opened := openEditorOnAServerOfNames(t, writeProject(t, map[string]string{
		"game.project":             "[project]\ntitle = Pickups\n",
		"pickups/level.collection": "name: \"level\"\ninstances {\n  id: \"coin\"\n  prototype: \"/pickups/coin.go\"\n}\n",
		"pickups/coin.go":          object,
		"pickups/feedback.script":  "local data = sys.load_resource(\"/pickups/pickups.atlas\")\n",
		"pickups/pickups.atlas":    "images {\n  image: \"/pickups/coin.png\"\n}\n",
		"pickups/coin.png":         "",
		"pickups/spare.lua":        "return {}\n",
	}))
	opened.start()
	opened.diagnosticsOf("pickups/spare.lua")
	fileOf := func(name string) string { return uriOf(opened.root + "/pickups/" + name) }

	cases := map[string]struct {
		from positionParams
		want string
	}{
		"the script of a component":                 {from: at(fileOf("coin.go"), 2, 25), want: fileOf("feedback.script")},
		"the atlas of a sprite written in place":    {from: at(fileOf("coin.go"), 7, 30), want: fileOf("pickups.atlas")},
		"the game object a collection places":       {from: at(fileOf("level.collection"), 3, 20), want: fileOf("coin.go")},
		"the image an atlas names":                  {from: at(fileOf("pickups.atlas"), 1, 15), want: fileOf("coin.png")},
		"a path written in a script":                {from: at(fileOf("feedback.script"), 0, 40), want: fileOf("pickups.atlas")},
		"the id of the component, which is no path": {from: at(fileOf("coin.go"), 1, 8)},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			leads := resultOf[[]location](t, opened.request("textDocument/definition", testCase.from))

			switch {
			case testCase.want == "" && len(leads) != 0:
				t.Errorf("leads to %+v, want nowhere", leads)
			case testCase.want != "" && (len(leads) != 1 || leads[0].URI != testCase.want):
				t.Errorf("leads to %+v, want %s", leads, testCase.want)
			}
		})
	}

	for _, method := range []string{"textDocument/prepareRename"} {
		answered := opened.request(method, at(fileOf("coin.go"), 2, 25))
		if answered.Error != nil || answered.Result != nil {
			t.Errorf("%s in a data file answered %v, %+v; want nothing", method, answered.Result, answered.Error)
		}
	}
}

// A data file written by hand gets the help a script gets, as far as it goes:
// what a value may be is offered inside its quotes, a path says what its file
// is, and a problem that knows what was meant is fixed by replacing the value,
// and only the value, where it is written plainly and where its quotes are
// escaped inside a component written in place.
func TestADataFileIsCompletedExplainedAndFixed(t *testing.T) {
	object := "components {\n  id: \"script\"\n  component: \"/pickups/feedbak.script\"\n}\nembedded_components {\n  id: \"sprite\"\n  type: \"sprite\"\n  data: \"tile_set: \\\"/pickups/pickups.atlas\\\"\\n\"\n  \"default_animation: \\\"cion\\\"\\n\"\n  \"\"\n}\n"
	opened := openEditorOnAServerOfNames(t, writeProject(t, map[string]string{
		"game.project":             "[project]\ntitle = Pickups\n",
		"pickups/level.collection": "name: \"level\"\ninstances {\n  id: \"coin\"\n  prototype: \"/pickups/coin.go\"\n}\n",
		"pickups/coin.go":          object,
		"pickups/feedback.script":  "function init(self)\nend\n",
		"pickups/pickups.atlas":    "images {\n  image: \"/pickups/coin.png\"\n}\n",
		"pickups/coin.png":         "",
	}))
	opened.start()
	document := uriOf(opened.root + "/pickups/coin.go")
	problems := opened.diagnosticsOf("pickups/coin.go")
	if len(problems) != 2 {
		t.Fatalf("problems = %+v, want the file that is not there and the animation the atlas does not have", problems)
	}

	offered := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 2, 20)))
	if len(offered) != 1 || offered[0].Label != "/pickups/feedback.script" || offered[0].Kind != kindOfAFile || offered[0].TextEdit.Range != (textRange{Start: position{Line: 2, Character: 14}, End: position{Line: 2, Character: 37}}) {
		t.Errorf("offered for the component = %+v, want the one script, replacing the whole path", offered)
	}
	animations := resultOf[[]completionItem](t, opened.request("textDocument/completion", at(document, 8, 26)))
	if len(animations) != 1 || animations[0].Label != "coin" || animations[0].Kind != kindOfAValue {
		t.Errorf("offered for the animation = %+v, want the one of the atlas", animations)
	}

	said := resultOf[hover](t, opened.request("textDocument/hover", at(document, 7, 30))).Contents.Value
	if !strings.Contains(said, "**/pickups/pickups.atlas**") || !strings.Contains(said, "1 animations: coin") {
		t.Errorf("the hover of the atlas says:\n%s", said)
	}

	fixed := []byte(object)
	for _, line := range []int{8, 2} {
		within := positionParams{TextDocument: textDocument{URI: document}, Range: textRange{Start: position{Line: line, Character: 0}, End: position{Line: line, Character: 40}}}
		fixes := resultOf[[]codeAction](t, opened.request("textDocument/codeAction", within))
		if len(fixes) != 1 || len(fixes[0].Edit.Changes[document]) != 1 {
			t.Fatalf("fixes on line %d = %+v", line+1, fixes)
		}
		edit := fixes[0].Edit.Changes[document][0]
		start, end := offsetAt(fixed, edit.Range.Start), offsetAt(fixed, edit.Range.End)
		fixed = slices.Concat(fixed[:start], []byte(edit.NewText), fixed[end:])
	}
	want := strings.NewReplacer("feedbak.script", "feedback.script", "cion", "coin").Replace(object)
	if string(fixed) != want {
		t.Errorf("after the fixes the file is\n%s\nwant\n%s", fixed, want)
	}
}

func gameWithFilesThatNameEachOther(t *testing.T) *editor {
	t.Helper()
	opened := openEditorOnAServerOfNames(t, writeProject(t, map[string]string{
		"game.project":             "[project]\ntitle = Pickups\n\n[bootstrap]\nmain_collection = /pickups/level.collectionc\n",
		"pickups/level.collection": "name: \"level\"\ninstances {\n  id: \"coin\"\n  prototype: \"/pickups/coin.go\"\n}\n",
		"pickups/coin.go":          "components {\n  id: \"script\"\n  component: \"/pickups/feedback.script\"\n}\n",
		"pickups/feedback.script":  "local names = require(\"pickups.names\")\n",
		"pickups/names.lua":        "return {}\n",
		"pickups/spare.lua":        "return {}\n",
	}))
	opened.start()
	opened.diagnosticsOf("pickups/spare.lua")
	return opened
}

// What uses a file is every place that names it. It is asked from a path, as
// the references of that path, or of the file that is open, by a request of
// this server's own, since an editor has no word for the file itself.
func TestWhatUsesAFileIsFound(t *testing.T) {
	opened := gameWithFilesThatNameEachOther(t)
	fileOf := func(name string) string { return uriOf(opened.root + "/pickups/" + name) }

	fromThePath := resultOf[[]location](t, opened.request("textDocument/references", at(fileOf("level.collection"), 3, 20)))
	ofTheFile := resultOf[[]location](t, opened.request("defold/uses", positionParams{TextDocument: textDocument{URI: fileOf("coin.go")}}))
	want := []location{{URI: fileOf("level.collection"), Range: textRange{Start: position{Line: 3, Character: 14}, End: position{Line: 3, Character: 30}}}}
	if !slices.Equal(fromThePath, want) || !slices.Equal(ofTheFile, want) {
		t.Errorf("from the path: %+v; of the file: %+v; want %+v", fromThePath, ofTheFile, want)
	}

	ofTheModule := resultOf[[]location](t, opened.request("defold/uses", positionParams{TextDocument: textDocument{URI: fileOf("names.lua")}}))
	if len(ofTheModule) != 1 || ofTheModule[0].URI != fileOf("feedback.script") || ofTheModule[0].Range.Start != (position{Line: 0, Character: 23}) {
		t.Errorf("a module is used by %+v, want the script that requires it", ofTheModule)
	}
	if none := resultOf[[]location](t, opened.request("defold/uses", positionParams{TextDocument: textDocument{URI: fileOf("spare.lua")}})); len(none) != 0 {
		t.Errorf("a file nothing names is used by %+v", none)
	}
}

// Before an editor moves or renames a file it asks what else has to change, and
// is given the edits that write the new path wherever the old one was, in what
// is open and not saved as in what is on disk. The editor applies them with the
// move, so that both happen or neither.
func TestMovingAFileGivesTheEditsThatFollowIt(t *testing.T) {
	opened := gameWithFilesThatNameEachOther(t)
	fileOf := func(name string) string { return uriOf(opened.root + "/" + name) }
	typed := "-- typed and not saved\nlocal names = require(\"pickups.names\")\n"
	opened.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocument{URI: fileOf("pickups/feedback.script"), Text: typed}})

	edits := resultOf[workspaceEdit](t, opened.request("workspace/willRenameFiles", renameFilesParams{Files: []renamedFile{
		{OldURI: fileOf("pickups/coin.go"), NewURI: fileOf("objects/coin.go")},
		{OldURI: fileOf("pickups/names.lua"), NewURI: fileOf("shared/ids.lua")},
		{OldURI: fileOf("pickups/level.collection"), NewURI: fileOf("levels/first.collection")},
	}})).Changes

	want := map[string][]textEdit{
		fileOf("pickups/level.collection"): {{Range: textRange{Start: position{Line: 3, Character: 14}, End: position{Line: 3, Character: 30}}, NewText: "/objects/coin.go"}},
		fileOf("pickups/feedback.script"):  {{Range: textRange{Start: position{Line: 1, Character: 23}, End: position{Line: 1, Character: 36}}, NewText: "shared.ids"}},
		fileOf("game.project"):             {{Range: textRange{Start: position{Line: 4, Character: 18}, End: position{Line: 4, Character: 44}}, NewText: "/levels/first.collectionc"}},
	}
	if len(edits) != len(want) {
		t.Fatalf("edits = %+v, want %+v", edits, want)
	}
	for file, wanted := range want {
		if !slices.Equal(edits[file], wanted) {
			t.Errorf("edits of %s = %+v, want %+v", file, edits[file], wanted)
		}
	}
}

// A move that would land on a file that is there, or that touches nothing the
// project names, changes nothing, and the first says why.
func TestAMoveThatCannotBeFollowedChangesNothing(t *testing.T) {
	opened := gameWithFilesThatNameEachOther(t)
	fileOf := func(name string) string { return uriOf(opened.root + "/" + name) }

	onto := opened.request("workspace/willRenameFiles", renameFilesParams{Files: []renamedFile{{OldURI: fileOf("pickups/coin.go"), NewURI: fileOf("pickups/names.lua")}}})
	if onto.Result != nil || len(opened.shown) != 1 || !strings.Contains(opened.shown[0].Message, "/pickups/names.lua is already there") {
		t.Errorf("a move onto a file answered %v and showed %+v", onto.Result, opened.shown)
	}
	elsewhere := opened.request("workspace/willRenameFiles", renameFilesParams{Files: []renamedFile{{OldURI: fileOf("notes/todo.txt"), NewURI: fileOf("notes/done.txt")}}})
	if elsewhere.Result != nil || elsewhere.Error != nil || len(opened.shown) != 1 {
		t.Errorf("a move of what the project does not hold answered %v, %+v", elsewhere.Result, elsewhere.Error)
	}
}
