package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/mobdebug"
)

// The stack the debugger of Defold 1.13.2 sent for a game stopped in update.
const stackOfAStoppedGame = `#lua/structure{:value #lua/ref"0x1" :refs {#lua/ref"0x1" #lua/table{:content [1 #lua/ref"0x2"] :string "table: 0x1"} #lua/ref"0x2" #lua/table{:content [1 #lua/ref"0x3" 2 #lua/ref"0x4" 3 #lua/ref"0x5"] :string "table: 0x2"} #lua/ref"0x3" #lua/table{:content [2 "main/ball.script" 3 1 4 3 5 "Lua" 6 "" 7 "main/ball.script"] :string "table: 0x3"} #lua/ref"0x4" #lua/table{:content ["dt" #lua/ref"0x6" "self" #lua/ref"0x7"] :string "table: 0x4"} #lua/ref"0x5" #lua/table{:content [] :string "table: 0x5"} #lua/ref"0x6" #lua/table{:content [1 0.016 2 "0.016"] :string "table: 0x6"} #lua/ref"0x7" #lua/table{:content [1 #lua/ref"Script: 0x8" 2 "Script: 0x8"] :string "table: 0x7"} #lua/ref"Script: 0x8" #lua/table{:content ["speed" 120] :string "Script: 0x8"}}}`

type editor struct {
	t       *testing.T
	toIt    io.Writer
	fromIt  *bufio.Reader
	sent    int
	events  []string
	stopped []string
}

func (e *editor) receive() message {
	e.t.Helper()
	type outcome struct {
		received message
		err      error
	}
	arrived := make(chan outcome, 1)
	go func() {
		received, err := received(e.fromIt)
		arrived <- outcome{received, err}
	}()
	select {
	case got := <-arrived:
		if got.err != nil {
			e.t.Fatalf("the adapter stopped answering: %v", got.err)
		}
		return got.received
	case <-time.After(10 * time.Second):
		e.t.Fatal("the adapter did not answer in time")
		return message{}
	}
}

func (e *editor) note(event message) {
	e.events = append(e.events, event.Event)
	if event.Event == "stopped" {
		encoded, _ := json.Marshal(event.Body)
		var body struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(encoded, &body)
		e.stopped = append(e.stopped, body.Reason)
	}
}

func (e *editor) ask(command string, arguments any, into any) message {
	e.t.Helper()
	e.sent++
	encoded, err := json.Marshal(arguments)
	if err != nil {
		e.t.Fatal(err)
	}
	request, _ := json.Marshal(message{Seq: e.sent, Type: "request", Command: command, Arguments: encoded})
	fmt.Fprintf(e.toIt, "Content-Length: %d\r\n\r\n%s", len(request), request)
	for {
		answered := e.receive()
		if answered.Type == "event" {
			e.note(answered)
			continue
		}
		if answered.RequestSeq != e.sent {
			continue
		}
		if into != nil && answered.Success {
			body, _ := json.Marshal(answered.Body)
			if err := json.Unmarshal(body, into); err != nil {
				e.t.Fatal(err)
			}
		}
		return answered
	}
}

func (e *editor) waitFor(event string) {
	e.t.Helper()
	for !slices.Contains(e.events, event) {
		received := e.receive()
		if received.Type == "event" {
			e.note(received)
		}
	}
	e.events = slices.DeleteFunc(e.events, func(seen string) bool { return seen == event })
}

func gameThatAnswers(t *testing.T, answers map[string]string) (address string, heard chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	heard = make(chan string, 64)
	t.Cleanup(func() { listener.Close() })
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		lines := bufio.NewScanner(connection)
		for lines.Scan() {
			command := lines.Text()
			heard <- command
			word, _, _ := strings.Cut(command, " ")
			fmt.Fprint(connection, answers[word])
		}
	}()
	return listener.Addr().String(), heard
}

// An editor debugs through the adapter the way it debugs anything: it sets breakpoints
// by file and line, is told when the game stops, and asks what is on the stack. The
// game on the other side speaks the engine's own debugger protocol.
func TestAnEditorDebugsAGameThroughTheAdapter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "pong")
	address, heard := gameThatAnswers(t, map[string]string{
		"SETB":  "200 OK\n",
		"DELB":  "200 OK\n",
		"RUN":   "200 OK\n202 Paused main/ball.script 3\n",
		"OVER":  "200 OK\n202 Paused main/ball.script 4\n",
		"STACK": "200 OK " + stackOfAStoppedGame + "\n",
		"EXEC":  "401 Error in Expression 9\nno speed!",
	})
	stoppedTheGame := make(chan struct{})
	server := &Server{Launch: func(ctx context.Context, asked LaunchArguments, say func(string)) (*Launched, error) {
		say("INFO:ENGINE: Defold Engine")
		session, err := mobdebug.Connect(ctx, address)
		return &Launched{Session: session, Stop: func() { close(stoppedTheGame) }, Exited: make(chan struct{})}, err
	}}
	toAdapter, fromEditor := io.Pipe()
	toEditor, fromAdapter := io.Pipe()
	served := make(chan error, 1)
	go func() { served <- server.Serve(toAdapter, fromAdapter) }()
	opened := &editor{t: t, toIt: fromEditor, fromIt: bufio.NewReader(toEditor)}

	var capabilities map[string]bool
	opened.ask("initialize", map[string]string{"adapterID": "defold"}, &capabilities)
	if !capabilities["supportsConfigurationDoneRequest"] {
		t.Errorf("capabilities = %v", capabilities)
	}
	if launched := opened.ask("launch", LaunchArguments{Root: root}, nil); !launched.Success {
		t.Fatalf("launch refused: %s", launched.Message)
	}
	opened.waitFor("initialized")

	var set struct {
		Breakpoints []struct {
			Verified bool `json:"verified"`
			Line     int  `json:"line"`
		} `json:"breakpoints"`
	}
	script := map[string]string{"path": filepath.Join(root, "main", "ball.script")}
	opened.ask("setBreakpoints", map[string]any{"source": script, "breakpoints": []map[string]int{{"line": 3}, {"line": 9}}}, &set)
	if len(set.Breakpoints) != 2 || !set.Breakpoints[0].Verified || set.Breakpoints[1].Line != 9 {
		t.Errorf("breakpoints = %+v", set.Breakpoints)
	}
	opened.ask("configurationDone", nil, nil)
	opened.waitFor("stopped")
	if commands := []string{<-heard, <-heard, <-heard}; !slices.Equal(commands, []string{"SETB main/ball.script 3", "SETB main/ball.script 9", "RUN"}) {
		t.Errorf("the game heard %q", commands)
	}

	var trace struct {
		StackFrames []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Line   int    `json:"line"`
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
		} `json:"stackFrames"`
	}
	opened.ask("stackTrace", map[string]int{"threadId": 1}, &trace)
	if len(trace.StackFrames) != 1 || trace.StackFrames[0].Line != 3 || trace.StackFrames[0].Source.Path != filepath.Join(root, "main", "ball.script") {
		t.Fatalf("stack = %+v", trace.StackFrames)
	}
	var scopes struct {
		Scopes []struct {
			Name      string `json:"name"`
			Reference int    `json:"variablesReference"`
		} `json:"scopes"`
	}
	opened.ask("scopes", map[string]int{"frameId": trace.StackFrames[0].ID}, &scopes)
	type variables struct {
		Variables []struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Reference int    `json:"variablesReference"`
		} `json:"variables"`
	}
	var locals, fields variables
	opened.ask("variables", map[string]int{"variablesReference": scopes.Scopes[0].Reference}, &locals)
	if len(locals.Variables) != 2 || locals.Variables[0].Name != "dt" || locals.Variables[0].Value != "0.016" || locals.Variables[1].Name != "self" || locals.Variables[1].Reference == 0 {
		t.Fatalf("locals = %+v", locals.Variables)
	}
	opened.ask("variables", map[string]int{"variablesReference": locals.Variables[1].Reference}, &fields)
	if len(fields.Variables) != 1 || fields.Variables[0].Name != "speed" || fields.Variables[0].Value != "120" {
		t.Errorf("the fields of self = %+v", fields.Variables)
	}
	if scopes.Scopes[1].Name != "Upvalues" || scopes.Scopes[1].Reference != 0 {
		t.Errorf("a scope that holds nothing can be opened: %+v", scopes.Scopes[1])
	}

	if refused := opened.ask("evaluate", map[string]any{"expression": "speed", "frameId": 1}, nil); refused.Success || refused.Message != "no speed!" {
		t.Errorf("a mistake in an expression is answered with %+v", refused)
	}

	opened.ask("next", map[string]int{"threadId": 1}, nil)
	opened.waitFor("stopped")
	if !slices.Equal(opened.stopped, []string{"breakpoint", "step"}) {
		t.Errorf("the reasons the game stopped for = %q", opened.stopped)
	}

	opened.ask("setBreakpoints", map[string]any{"source": script, "breakpoints": []map[string]int{{"line": 9}}}, nil)
	for command := range heard {
		if command == "DELB main/ball.script 3" {
			break
		}
	}

	opened.ask("disconnect", nil, nil)
	opened.waitFor("terminated")
	select {
	case <-stoppedTheGame:
	case <-time.After(5 * time.Second):
		t.Error("the game was left running")
	}
	if err := <-served; err != nil {
		t.Errorf("the adapter ended with %v", err)
	}
}

func TestAGameThatCannotBeStartedIsRefusedWithTheReason(t *testing.T) {
	server := &Server{Launch: func(context.Context, LaunchArguments, func(string)) (*Launched, error) {
		return nil, errors.New("no Defold editor was found")
	}}
	toAdapter, fromEditor := io.Pipe()
	toEditor, fromAdapter := io.Pipe()
	go func() { _ = server.Serve(toAdapter, fromAdapter) }()
	opened := &editor{t: t, toIt: fromEditor, fromIt: bufio.NewReader(toEditor)}

	if refused := opened.ask("launch", LaunchArguments{Root: t.TempDir()}, nil); refused.Success || refused.Message != "no Defold editor was found" {
		t.Errorf("answer = %+v", refused)
	}
	if nothing := opened.ask("stackTrace", map[string]int{"threadId": 1}, nil); nothing.Success {
		t.Errorf("a stack was given with no game: %+v", nothing)
	}
	fromEditor.Close()
}
