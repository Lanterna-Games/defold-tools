package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Lanterna-Games/defold-tools/internal/mobdebug"
)

const (
	theOnlyThread     = 1
	nameOfTheThread   = "main"
	headerOfTheLength = "Content-Length: "
	scopeOfLocals     = "Locals"
	scopeOfUpvalues   = "Upvalues"

	stoppedAtABreakpoint = "breakpoint"
	stoppedAfterAStep    = "step"
	stoppedByAPause      = "pause"
)

type LaunchArguments struct {
	Root       string `json:"root"`
	DefoldHome string `json:"defoldHome"`
	Engine     string `json:"engine"`
	IsBuilt    bool   `json:"built"`
}

type Launched struct {
	Session *mobdebug.Session
	Stop    func()
	Exited  <-chan struct{}
}

type Server struct {
	Launch func(ctx context.Context, asked LaunchArguments, say func(text string)) (*Launched, error)

	out     io.Writer
	writing sync.Mutex
	sent    int

	state        sync.Mutex
	root         string
	game         *Launched
	breakpoints  map[string][]int
	frames       []mobdebug.Frame
	held         [][]mobdebug.Named
	lastStop     mobdebug.Stop
	lastWasAStep bool
	quietPause   chan struct{}
	wasAsked     bool
}

type message struct {
	Seq        int             `json:"seq"`
	Type       string          `json:"type"`
	Command    string          `json:"command,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	RequestSeq int             `json:"request_seq,omitempty"`
	Success    bool            `json:"success"`
	Message    string          `json:"message,omitempty"`
	Event      string          `json:"event,omitempty"`
	Body       any             `json:"body,omitempty"`
}

func (s *Server) send(sent message) {
	s.writing.Lock()
	defer s.writing.Unlock()
	s.sent++
	sent.Seq = s.sent
	encoded, err := json.Marshal(sent)
	if err != nil {
		return
	}
	fmt.Fprintf(s.out, "%s%d\r\n\r\n%s", headerOfTheLength, len(encoded), encoded)
}

func (s *Server) event(name string, body any) {
	s.send(message{Type: "event", Event: name, Body: body})
}

func (s *Server) answer(request message, body any) {
	s.send(message{Type: "response", RequestSeq: request.Seq, Command: request.Command, Success: true, Body: body})
}

func (s *Server) refuse(request message, err error) {
	s.send(message{Type: "response", RequestSeq: request.Seq, Command: request.Command, Success: false, Message: err.Error()})
}

func (s *Server) say(text string) {
	s.event("output", map[string]string{"category": "stdout", "output": text + "\n"})
}

func received(in *bufio.Reader) (message, error) {
	length := 0
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return message{}, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if written, isTheLength := strings.CutPrefix(line, strings.TrimSpace(headerOfTheLength)); isTheLength {
			length, _ = strconv.Atoi(strings.TrimSpace(written))
		}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(in, body); err != nil {
		return message{}, err
	}
	var request message
	return request, json.Unmarshal(body, &request)
}

func (s *Server) Serve(in io.Reader, out io.Writer) error {
	s.out, s.breakpoints = out, map[string][]int{}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	reader := bufio.NewReader(in)
	for {
		request, err := received(reader)
		if errors.Is(err, io.EOF) {
			s.leave()
			return nil
		}
		if err != nil {
			s.leave()
			return err
		}
		if isTheLast := s.handle(ctx, request); isTheLast {
			return nil
		}
	}
}

func (s *Server) leave() {
	s.state.Lock()
	game := s.game
	s.game = nil
	s.state.Unlock()
	if game != nil {
		game.Session.Close()
		game.Stop()
	}
}

type sourceOfAFile struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path"`
}

func (s *Server) nameInTheGame(path string) string {
	relative, err := filepath.Rel(s.root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func (s *Server) handle(ctx context.Context, request message) (isTheLast bool) {
	switch request.Command {
	case "initialize":
		s.answer(request, map[string]bool{"supportsConfigurationDoneRequest": true, "supportsEvaluateForHovers": true})
	case "launch":
		s.launch(ctx, request)
	case "setBreakpoints":
		s.setBreakpoints(request)
	case "configurationDone":
		s.resume(request, mobdebug.CommandToRun, nil)
	case "threads":
		s.answer(request, map[string]any{"threads": []map[string]any{{"id": theOnlyThread, "name": nameOfTheThread}}})
	case "stackTrace":
		s.stackTrace(request)
	case "scopes":
		s.scopes(request)
	case "variables":
		s.variables(request)
	case "continue":
		s.resume(request, mobdebug.CommandToRun, map[string]bool{"allThreadsContinued": true})
	case "next":
		s.resume(request, mobdebug.CommandToStepOver, nil)
	case "stepIn":
		s.resume(request, mobdebug.CommandToStepIn, nil)
	case "stepOut":
		s.resume(request, mobdebug.CommandToStepOut, nil)
	case "pause":
		s.pause(request)
	case "evaluate":
		s.evaluate(request)
	case "disconnect", "terminate":
		s.leave()
		s.answer(request, nil)
		s.event("terminated", nil)
		return true
	default:
		s.answer(request, nil)
	}
	return false
}

var errNoGame = errors.New("no game is being debugged")

func (s *Server) session() (*mobdebug.Session, error) {
	s.state.Lock()
	defer s.state.Unlock()
	if s.game == nil {
		return nil, errNoGame
	}
	return s.game.Session, nil
}

func (s *Server) launch(ctx context.Context, request message) {
	var asked LaunchArguments
	if err := json.Unmarshal(request.Arguments, &asked); err != nil {
		s.refuse(request, err)
		return
	}
	game, err := s.Launch(ctx, asked, s.say)
	if err != nil {
		s.refuse(request, err)
		return
	}
	s.state.Lock()
	s.game, s.root = game, asked.Root
	s.state.Unlock()
	go func() {
		<-game.Exited
		s.event("terminated", nil)
	}()
	s.answer(request, nil)
	s.event("initialized", nil)
}

func (s *Server) watch(session *mobdebug.Session, command string) {
	for {
		stop, err := session.WaitForAStop()
		if err != nil {
			return
		}
		s.state.Lock()
		quiet := s.quietPause
		s.quietPause = nil
		if quiet != nil {
			s.state.Unlock()
			close(quiet)
			return
		}
		isAStep := command != mobdebug.CommandToRun
		staysWhereItWas := isAStep && !s.lastWasAStep && stop == s.lastStop
		s.lastStop, s.frames, s.held = stop, nil, nil
		if !staysWhereItWas {
			s.lastWasAStep = isAStep
		}
		s.state.Unlock()
		if staysWhereItWas {
			s.state.Lock()
			s.lastWasAStep = true
			s.state.Unlock()
			if err := session.Resume(command); err != nil {
				return
			}
			continue
		}
		s.state.Lock()
		wasAsked := s.wasAsked
		s.wasAsked = false
		s.state.Unlock()
		reason := stoppedAtABreakpoint
		switch {
		case wasAsked:
			reason = stoppedByAPause
		case isAStep:
			reason = stoppedAfterAStep
		}
		s.event("stopped", map[string]any{"reason": reason, "threadId": theOnlyThread, "allThreadsStopped": true})
		return
	}
}

func (s *Server) resume(request message, command string, body any) {
	session, err := s.session()
	if err == nil {
		err = session.Resume(command)
	}
	if err != nil {
		s.refuse(request, err)
		return
	}
	s.answer(request, body)
	go s.watch(session, command)
}

func (s *Server) pause(request message) {
	session, err := s.session()
	if err == nil {
		s.state.Lock()
		s.wasAsked = true
		s.state.Unlock()
		err = session.Pause()
	}
	if err != nil {
		s.refuse(request, err)
		return
	}
	s.answer(request, nil)
}

func (s *Server) setBreakpoints(request message) {
	var asked struct {
		Source      sourceOfAFile `json:"source"`
		Breakpoints []struct {
			Line int `json:"line"`
		} `json:"breakpoints"`
	}
	session, err := s.session()
	if err == nil {
		err = json.Unmarshal(request.Arguments, &asked)
	}
	if err != nil {
		s.refuse(request, err)
		return
	}
	file := s.nameInTheGame(asked.Source.Path)
	wanted := make([]int, 0, len(asked.Breakpoints))
	for _, breakpoint := range asked.Breakpoints {
		wanted = append(wanted, breakpoint.Line)
	}
	s.state.Lock()
	set := s.breakpoints[file]
	s.state.Unlock()
	wasRunning := !session.IsPaused()
	if wasRunning {
		paused := make(chan struct{})
		s.state.Lock()
		s.quietPause = paused
		s.state.Unlock()
		if err = session.Pause(); err == nil {
			<-paused
		}
	}
	for _, line := range set {
		if err == nil && !slices.Contains(wanted, line) {
			err = session.RemoveBreakpoint(file, line)
		}
	}
	for _, line := range wanted {
		if err == nil && !slices.Contains(set, line) {
			err = session.SetBreakpoint(file, line)
		}
	}
	if wasRunning && err == nil {
		if err = session.Resume(mobdebug.CommandToRun); err == nil {
			go s.watch(session, mobdebug.CommandToRun)
		}
	}
	if err != nil {
		s.refuse(request, err)
		return
	}
	s.state.Lock()
	s.breakpoints[file] = wanted
	s.state.Unlock()
	verified := make([]map[string]any, 0, len(wanted))
	for _, line := range wanted {
		verified = append(verified, map[string]any{"verified": true, "line": line})
	}
	s.answer(request, map[string]any{"breakpoints": verified})
}

func (s *Server) framesNow() ([]mobdebug.Frame, error) {
	session, err := s.session()
	if err != nil {
		return nil, err
	}
	s.state.Lock()
	defer s.state.Unlock()
	if s.frames == nil {
		if s.frames, err = session.Stack(); err != nil {
			return nil, err
		}
	}
	return s.frames, nil
}

func (s *Server) stackTrace(request message) {
	frames, err := s.framesNow()
	if err != nil {
		s.refuse(request, err)
		return
	}
	shown := make([]map[string]any, 0, len(frames))
	for index, frame := range frames {
		name := frame.Name
		if name == "" {
			name = filepath.Base(frame.File)
		}
		shown = append(shown, map[string]any{
			"id": index + 1, "name": name, "line": frame.Line, "column": 1,
			"source": sourceOfAFile{Name: filepath.Base(frame.File), Path: filepath.Join(s.root, filepath.FromSlash(frame.File))},
		})
	}
	s.answer(request, map[string]any{"stackFrames": shown, "totalFrames": len(shown)})
}

func (s *Server) hold(variables []mobdebug.Named) int {
	if len(variables) == 0 {
		return 0
	}
	s.state.Lock()
	defer s.state.Unlock()
	s.held = append(s.held, variables)
	return len(s.held)
}

func (s *Server) scopes(request message) {
	var asked struct {
		FrameID int `json:"frameId"`
	}
	frames, err := s.framesNow()
	if err == nil {
		err = json.Unmarshal(request.Arguments, &asked)
	}
	if err != nil || asked.FrameID < 1 || asked.FrameID > len(frames) {
		s.refuse(request, errors.Join(err, errors.New("the game has moved on from that frame")))
		return
	}
	frame := frames[asked.FrameID-1]
	s.answer(request, map[string]any{"scopes": []map[string]any{
		{"name": scopeOfLocals, "variablesReference": s.hold(frame.Locals), "expensive": false},
		{"name": scopeOfUpvalues, "variablesReference": s.hold(frame.Upvalues), "expensive": false},
	}})
}

func (s *Server) shown(named mobdebug.Named) map[string]any {
	return map[string]any{"name": named.Name, "value": named.Value.Text, "type": named.Value.Kind, "variablesReference": s.hold(named.Value.Children)}
}

func (s *Server) variables(request message) {
	var asked struct {
		Reference int `json:"variablesReference"`
	}
	_ = json.Unmarshal(request.Arguments, &asked)
	s.state.Lock()
	var held []mobdebug.Named
	if asked.Reference >= 1 && asked.Reference <= len(s.held) {
		held = s.held[asked.Reference-1]
	}
	s.state.Unlock()
	shown := make([]map[string]any, 0, len(held))
	for _, each := range held {
		shown = append(shown, s.shown(each))
	}
	s.answer(request, map[string]any{"variables": shown})
}

func (s *Server) evaluate(request message) {
	var asked struct {
		Expression string `json:"expression"`
		FrameID    int    `json:"frameId"`
	}
	session, err := s.session()
	if err == nil {
		err = json.Unmarshal(request.Arguments, &asked)
	}
	if err != nil {
		s.refuse(request, err)
		return
	}
	value, err := session.Evaluate(asked.Expression, max(asked.FrameID, 1))
	if err != nil {
		s.refuse(request, err)
		return
	}
	s.answer(request, map[string]any{"result": value.Text, "type": value.Kind, "variablesReference": s.hold(value.Children)})
}
