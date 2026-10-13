package lsp

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/analysis"
	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/insight"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	pauseBeforeCheck = 200 * time.Millisecond
)

type Session struct {
	Root   string
	server *Server
}

func (s Session) Notify(method string, params any) {
	_ = s.server.connection.notify(method, params)
}

func (s Session) Consult(documentation insight.Documentation) {
	s.server.state.Lock()
	s.server.documentation = documentation
	s.server.state.Unlock()
}

func (s Session) Say(text string) {
	s.server.say(text)
}

func (s Session) Complain(err error) {
	s.server.complain(err)
}

type Server struct {
	Name     string
	Version  string
	Versions func(announce func(message string)) engine.Versions
	Pause    time.Duration
	Ready    func(session Session)

	AnswersAboutNames bool

	connection *connection

	state         sync.Mutex
	root          string
	unsaved       map[string][]byte
	reported      map[string]bool
	analysed      analysis.Result
	documentation insight.Documentation
	pending       *time.Timer
	checking      sync.Mutex
	readBefore    *project.Parsed
	isInitialized bool
}

func (s *Server) Serve(input io.Reader, output io.Writer) error {
	s.connection = newConnection(input, output)
	s.unsaved, s.reported = map[string][]byte{}, map[string]bool{}
	if s.Pause == 0 {
		s.Pause = pauseBeforeCheck
	}
	for {
		received, err := s.connection.read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if received.Method == "exit" {
			return nil
		}
		if err := s.handle(received); err != nil {
			return err
		}
	}
}

func (s *Server) handle(received *message) error {
	isARequest := received.ID != nil
	switch {
	case received.Method == "initialize":
		return s.initialize(received)
	case !s.isInitialized && isARequest:
		return s.connection.refuse(received.ID, serverNotStarted, "the server was not initialized")
	case !s.isInitialized:
		return nil
	}
	switch received.Method {
	case "initialized":
		s.checkSoon()
		if s.Ready != nil && s.root != "" {
			go s.Ready(Session{Root: s.root, server: s})
		}
	case "shutdown":
		return s.connection.answer(received.ID, nil)
	case "textDocument/didOpen":
		var params didOpenParams
		if json.Unmarshal(received.Params, &params) == nil {
			s.keep(params.TextDocument.URI, []byte(params.TextDocument.Text))
		}
	case "textDocument/didChange":
		var params didChangeParams
		if json.Unmarshal(received.Params, &params) == nil && len(params.ContentChanges) > 0 {
			s.keep(params.TextDocument.URI, []byte(params.ContentChanges[len(params.ContentChanges)-1].Text))
		}
	case "textDocument/didClose":
		var params didCloseParams
		if json.Unmarshal(received.Params, &params) == nil {
			s.forget(params.TextDocument.URI)
		}
	case "textDocument/didSave", "workspace/didChangeWatchedFiles":
		s.checkSoon()
	default:
		if s.AnswersAboutNames && isARequest && received.Method == methodThatMovesFiles {
			return s.moveFiles(received)
		}
		if s.AnswersAboutNames && isARequest && received.Method == methodThatSaysUses {
			return s.usesOf(received)
		}
		if answer, isAnswered := s.whatAnswers(received.Method); isAnswered {
			return s.answerAbout(received, answer)
		}
		if isARequest {
			return s.connection.refuse(received.ID, methodNotFound, received.Method+" is not something this server does")
		}
	}
	return nil
}

func (s *Server) initialize(received *message) error {
	if received.ID == nil {
		return nil
	}
	var params initializeParams
	if err := json.Unmarshal(received.Params, &params); err != nil {
		return s.connection.refuse(received.ID, requestWasInvalid, err.Error())
	}
	s.root = rootOf(params)
	s.isInitialized = true
	capabilities := serverCapabilities{TextDocumentSync: textDocumentSync{OpenClose: true, Change: syncFullText}}
	if s.AnswersAboutNames {
		capabilities.CompletionProvider = &completionOptions{TriggerCharacters: charactersThatStartAName}
		capabilities.DefinitionProvider = true
		capabilities.HoverProvider = true
		capabilities.CodeActionProvider = true
		capabilities.ReferencesProvider = true
		capabilities.RenameProvider = &renameOptions{PrepareProvider: true}
		capabilities.Workspace = &workspaceOptions{FileOperations: fileOperations{WillRename: fileOperationRegistration{
			Filters: []fileOperationFilter{{Scheme: schemeOfAFile, Pattern: fileOperationPattern{Glob: everyFile}}},
		}}}
	}
	return s.connection.answer(received.ID, initializeResult{
		Capabilities: capabilities,
		ServerInfo:   serverInfo{Name: s.Name, Version: s.Version},
	})
}

func (s *Server) whatAnswers(method string) (answer func(asked) any, isAnswered bool) {
	if !s.AnswersAboutNames {
		return nil, false
	}
	switch method {
	case "textDocument/completion":
		return s.completion, true
	case methodThatLeadsToAFile:
		return s.definition, true
	case "textDocument/hover":
		return s.hover, true
	case "textDocument/codeAction":
		return s.codeActions, true
	case "textDocument/references":
		return s.references, true
	case "textDocument/prepareRename":
		return s.whatCanBeRenamed, true
	case "textDocument/rename":
		return s.rename, true
	}
	return nil, false
}

func rootOf(params initializeParams) string {
	candidates := []string{params.RootURI}
	for _, folder := range params.WorkspaceFolders {
		candidates = append(candidates, folder.URI)
	}
	for _, candidate := range candidates {
		if root, isAFile := pathOf(candidate); isAFile {
			return root
		}
	}
	return filepath.ToSlash(params.RootPath)
}

func (s *Server) keep(uri string, content []byte) {
	if relative, isInTheProject := s.relative(uri); isInTheProject {
		s.state.Lock()
		s.unsaved[relative] = content
		s.state.Unlock()
		s.checkSoon()
	}
}

func (s *Server) forget(uri string) {
	if relative, isInTheProject := s.relative(uri); isInTheProject {
		s.state.Lock()
		delete(s.unsaved, relative)
		s.state.Unlock()
		s.checkSoon()
	}
}

func (s *Server) relative(uri string) (relative string, isInTheProject bool) {
	slashPath, isAFile := pathOf(uri)
	if !isAFile || s.root == "" {
		return "", false
	}
	return relativeTo(s.root, slashPath)
}

func (s *Server) checkSoon() {
	s.state.Lock()
	defer s.state.Unlock()
	if s.pending != nil {
		s.pending.Stop()
	}
	s.pending = time.AfterFunc(s.Pause, s.check)
}

func (s *Server) check() {
	s.checking.Lock()
	defer s.checking.Unlock()

	s.state.Lock()
	root := s.root
	files := overlay{disk: os.DirFS(filepath.FromSlash(root)), unsaved: maps.Clone(s.unsaved)}
	s.state.Unlock()
	if root == "" || !analysis.IsAProject(files) {
		return
	}

	if s.readBefore == nil {
		s.readBefore = project.NewParsed()
	}
	analysed, err := analysis.Run(files, s.Versions(s.say), analysis.Options{ReadBefore: s.readBefore})
	if err != nil {
		s.complain(err)
		analysed = analysis.Result{}
	}
	s.state.Lock()
	s.analysed = analysed
	s.state.Unlock()
	s.publish(root, files, analysed.Problems)
}

func (s *Server) say(text string) {
	_ = s.connection.notify("window/logMessage", showMessageParams{Type: messageIsInfo, Message: text})
}

func (s *Server) warn(text string) {
	_ = s.connection.notify("window/showMessage", showMessageParams{Type: messageIsWarning, Message: text})
}

func (s *Server) complain(err error) {
	_ = s.connection.notify("window/showMessage", showMessageParams{Type: messageIsError, Message: s.Name + ": " + err.Error()})
}

func diagnosticOf(problem check.Problem, content []byte) diagnostic {
	severity := severityError
	if !check.IsAMistake(problem.Check) {
		severity = severityWarning
	}
	return diagnostic{
		Range:    rangeOfAProblem(content, problem.Location.Position.Line, problem.Location.Position.Column),
		Severity: severity,
		Code:     problem.Check,
		Source:   sourceOfOurs,
		Message:  problem.Message,
	}
}

func (s *Server) publish(root string, files fs.FS, problems []check.Problem) {
	byFile := map[string][]diagnostic{}
	contents := map[string][]byte{}
	for _, problem := range problems {
		file := problem.Location.File
		if _, isRead := contents[file]; !isRead {
			contents[file] = contentOf(files, file)
		}
		byFile[file] = append(byFile[file], diagnosticOf(problem, contents[file]))
	}

	s.state.Lock()
	for file := range s.reported {
		if _, stillHasProblems := byFile[file]; !stillHasProblems {
			byFile[file] = []diagnostic{}
		}
	}
	s.reported = map[string]bool{}
	for file, diagnostics := range byFile {
		if len(diagnostics) > 0 {
			s.reported[file] = true
		}
	}
	s.state.Unlock()

	for _, file := range slices.Sorted(maps.Keys(byFile)) {
		_ = s.connection.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
			URI:         uriOf(root + "/" + file),
			Diagnostics: byFile[file],
		})
	}
}
