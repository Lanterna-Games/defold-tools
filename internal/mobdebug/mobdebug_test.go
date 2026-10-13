package mobdebug

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
)

// What the debugger of Defold 1.13.2 answered when a game was stopped in update, at
// line 18 of example/tint.script, and when it was asked for a table of several kinds
// of value. Both are kept as they came.
const (
	stackOfAStoppedGame = `#lua/structure{:value #lua/ref"0x79e52a856ec0" :refs {#lua/ref"0x79e52a8277a0" #lua/table{:content [] :string "table: 0x79e52a8277a0"} #lua/ref"Script: 0x79e52a81d750" #lua/table{:content ["time" 74.218579008462 "doubled" 148.37049201733] :string "Script: 0x79e52a81d750"} #lua/ref"0x79e52a827620" #lua/table{:content [1 #lua/ref"Script: 0x79e52a81d750" 2 "Script: 0x79e52a81d750"] :string "table: 0x79e52a827620"} #lua/ref"0x79e52a827700" #lua/table{:content [1 0.033332999795675 2 "0.033332999795675"] :string "table: 0x79e52a827700"} #lua/ref"0x79e52a856ec0" #lua/table{:content [1 #lua/ref"0x79e52a856f60"] :string "table: 0x79e52a856ec0"} #lua/ref"0x79e52a827360" #lua/table{:content [2 "example/tint.script" 3 16 4 18 5 "Lua" 6 "" 7 "example/tint.script"] :string "table: 0x79e52a827360"} #lua/ref"0x79e52a856f60" #lua/table{:content [1 #lua/ref"0x79e52a827360" 2 #lua/ref"0x79e52a8275d0" 3 #lua/ref"0x79e52a8277a0"] :string "table: 0x79e52a856f60"} #lua/ref"0x79e52a8275d0" #lua/table{:content ["dt" #lua/ref"0x79e52a827700" "self" #lua/ref"0x79e52a827620"] :string "table: 0x79e52a8275d0"}}}`
	valueOfSeveralKinds = `#lua/structure{:value #lua/ref"0x79e52a83daf0" :refs {#lua/ref"0x79e52a83daf0" #lua/table{:content [1 #lua/ref"0x79e52a8135d0"] :string "table: 0x79e52a83daf0"} #lua/ref"0x79e52a8136a0" #lua/table{:content ["nested" true] :string "table: 0x79e52a8136a0"} #lua/ref"0x79e52a8135d0" #lua/table{:content [1 74.218579008462 2 "text" 3 #lua/ref"0x79e52a8136a0" 4 #lua/userdata"vmath.vector3(1, 2, 3)" 5 #lua/userdata"hash: [x]" 7 #lua/userdata"hash: [/go]"] :string "table: 0x79e52a8135d0"}}}`
)

func described(variables []Named) []string {
	var lines []string
	for _, variable := range variables {
		lines = append(lines, variable.Name+" = "+variable.Value.Text+" ("+variable.Value.Kind+")")
		for _, child := range variable.Value.Children {
			lines = append(lines, "  "+child.Name+" = "+child.Value.Text)
		}
	}
	return lines
}

type scriptedDebugger struct {
	listener net.Listener
	heard    chan string
}

func debuggerThatAnswers(t *testing.T, answers map[string]string) *scriptedDebugger {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	scripted := &scriptedDebugger{listener: listener, heard: make(chan string, 64)}
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
			scripted.heard <- command
			word, _, _ := strings.Cut(command, " ")
			if answer, isKnown := answers[command]; isKnown {
				fmt.Fprint(connection, answer)
			} else if answer, isKnown := answers[word]; isKnown {
				fmt.Fprint(connection, answer)
			}
		}
	}()
	return scripted
}

func attachedTo(t *testing.T, scripted *scriptedDebugger) *Session {
	t.Helper()
	session, err := Connect(t.Context(), scripted.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func TestAStoppedGameSaysWhereItIsAndWhatItHolds(t *testing.T) {
	scripted := debuggerThatAnswers(t, map[string]string{
		"SETB":  "200 OK\n",
		"RUN":   "200 OK\n202 Paused example/tint.script 18\n",
		"STACK": "200 OK " + stackOfAStoppedGame + "\n",
	})
	session := attachedTo(t, scripted)

	if err := session.SetBreakpoint("example/tint.script", 18); err != nil {
		t.Fatal(err)
	}
	if err := session.Resume(CommandToRun); err != nil {
		t.Fatal(err)
	}
	stop, err := session.WaitForAStop()
	if err != nil || stop != (Stop{File: "example/tint.script", Line: 18}) {
		t.Fatalf("stop = %+v, %v", stop, err)
	}
	frames, err := session.Stack()
	if err != nil || len(frames) != 1 {
		t.Fatalf("frames = %+v, %v", frames, err)
	}
	if frame := frames[0]; frame.File != "example/tint.script" || frame.Line != 18 {
		t.Errorf("frame = %+v", frame)
	}
	wanted := []string{"dt = 0.033332999795675 (number)", "self = Script: 0x79e52a81d750 (table)", "  doubled = 148.37049201733", "  time = 74.218579008462"}
	if got := described(frames[0].Locals); !slices.Equal(got, wanted) {
		t.Errorf("locals = %q, want %q", got, wanted)
	}
	if heard := []string{<-scripted.heard, <-scripted.heard, <-scripted.heard}; !slices.Equal(heard, []string{"SETB example/tint.script 18", "RUN", "STACK --{maxlevel=3}"}) {
		t.Errorf("the debugger heard %q", heard)
	}
}

func TestAnExpressionIsEvaluatedWhereTheGameStopped(t *testing.T) {
	scripted := debuggerThatAnswers(t, map[string]string{
		"EXEC return values --{stack=1}":       fmt.Sprintf("200 OK %d\n%s", len(valueOfSeveralKinds), valueOfSeveralKinds),
		"EXEC return nothing.here --{stack=2}": "401 Error in Expression 34\nattempt to index global 'nothing'!",
	})
	session := attachedTo(t, scripted)

	value, err := session.Evaluate("values", 1)

	if err != nil {
		t.Fatal(err)
	}
	wanted := []string{
		"values = table: 0x79e52a8135d0 (table)", "  1 = 74.218579008462", "  2 = \"text\"", "  3 = table: 0x79e52a8136a0",
		"  4 = vmath.vector3(1, 2, 3)", "  5 = hash: [x]", "  7 = hash: [/go]",
	}
	if got := described([]Named{{Name: "values", Value: value}}); !slices.Equal(got, wanted) {
		t.Errorf("value = %q, want %q", got, wanted)
	}
	if _, err := session.Evaluate("nothing.here", 2); err == nil || !strings.Contains(err.Error(), "attempt to index global 'nothing'!") {
		t.Errorf("error = %v, want what the game said", err)
	}
}

// A pause is never answered: the game just stops. It says where when it was stepping
// line by line, and only that the run has ended, with the name of what it was in, when
// it was running freely. Both were seen from the real engine.
func TestAPauseIsAskedForWhileTheGameRuns(t *testing.T) {
	freely := debuggerThatAnswers(t, map[string]string{"RUN": "200 OK\n", "SUSPEND": "401 Error in Execution 3\n[C]", "SETB": "200 OK\n"})
	running := attachedTo(t, freely)
	if err := running.Resume(CommandToRun); err != nil {
		t.Fatal(err)
	}
	if err := running.SetBreakpoint("main/ball.script", 3); !errors.Is(err, ErrIsRunning) {
		t.Errorf("error = %v for a breakpoint set while the game runs, want it to say the game has to be paused", err)
	}
	if _, err := running.Stack(); !errors.Is(err, ErrIsRunning) {
		t.Errorf("error = %v for the stack of a game that runs, want it to say the game has to be paused", err)
	}
	if err := running.Pause(); err != nil {
		t.Fatal(err)
	}
	if stop, err := running.WaitForAStop(); err != nil || stop != (Stop{}) {
		t.Fatalf("stop = %+v, %v; want a stop that does not say where", stop, err)
	}
	if err := running.SetBreakpoint("main/ball.script", 3); err != nil {
		t.Errorf("after the pause a breakpoint is refused: %v", err)
	}

	scripted := debuggerThatAnswers(t, map[string]string{
		"RUN":     "200 OK\n",
		"SUSPEND": "202 Paused builtins/render/default.render_script 247\n",
		"OVER":    "200 OK\n202 Paused builtins/render/default.render_script 248\n",
	})
	session := attachedTo(t, scripted)
	if err := session.Resume(CommandToRun); err != nil {
		t.Fatal(err)
	}

	if err := session.Pause(); err != nil {
		t.Fatal(err)
	}
	stop, err := session.WaitForAStop()
	if err != nil || stop.File != "builtins/render/default.render_script" || stop.Line != 247 {
		t.Fatalf("stop = %+v, %v", stop, err)
	}
	if err := session.Resume(CommandToStepOver); err != nil {
		t.Fatalf("after the pause the next command is refused: %v", err)
	}
	if next, err := session.WaitForAStop(); err != nil || next.Line != 248 {
		t.Errorf("stop = %+v, %v: the answer to the pause was taken for the next stop", next, err)
	}
}

func TestTheMessageThatStartsTheDebuggerNamesTheModuleItNeeds(t *testing.T) {
	message := string(messageToStartTheDebugger(45123))
	for _, part := range []string{"require('builtins.scripts.debugger').start(45123)", "builtins.scripts.debugger", "/builtins/scripts/debugger.luac"} {
		if !strings.Contains(message, part) {
			t.Errorf("the message lacks %q", part)
		}
	}
	if port, err := FreePort(t.Context()); err != nil || port == 0 {
		t.Errorf("port = %d, %v", port, err)
	}
}
