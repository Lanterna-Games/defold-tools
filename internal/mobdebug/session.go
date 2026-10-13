package mobdebug

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	addressToRunAScript  = "http://127.0.0.1:%d/post/@system/run_script"
	scriptThatStartsIt   = "require('builtins.scripts.debugger').start(%d)"
	nameOfTheScript      = "=defold-tools/attach"
	ModuleOfTheDebugger  = "builtins.scripts.debugger"
	ResourceOfTheModule  = "/builtins/scripts/debugger.luac"
	timeToConnect        = 5 * time.Second
	pauseBetweenAttempts = 100 * time.Millisecond
	timeToSettle         = 500 * time.Millisecond
	timeToAnswer         = 10 * time.Second
	endOfARun            = "401 Error in Execution"
	outputOfTheGame      = "204"
	depthOfValues        = 3
	indexOfTheInfo       = 1
	indexOfTheLocals     = 2
	indexOfTheUpvalues   = 3
	indexOfTheName       = 1
	indexOfTheSource     = 2
	indexOfTheLine       = 4

	CommandToRun      = "RUN"
	CommandToStepIn   = "STEP"
	CommandToStepOver = "OVER"
	CommandToStepOut  = "OUT"
)

var (
	paused       = regexp.MustCompile(`^20[23] Paused\s+(.+?)\s+(\d+)(?:\s+\d+)?\s*$`)
	withAPayload = regexp.MustCompile(`^(?:200 OK|401 Error in \w+|204 Output \w+) (\d+)\s*$`)
	ErrIsRunning = errors.New("the game is running: it has to be paused first")
	ErrHasLeft   = errors.New("the game is no longer there")
)

type reply struct {
	line    string
	payload string
}

type Session struct {
	connection net.Conn
	asking     sync.Mutex
	replies    chan reply
	stops      chan Stop
	state      sync.Mutex
	isRunning  bool
}

type Stop struct {
	File string
	Line int
}

type Frame struct {
	Name     string
	File     string
	Line     int
	Locals   []Named
	Upvalues []Named
}

func field(number byte, payload []byte) []byte {
	encoded := []byte{number<<3 | 2}
	length := len(payload)
	for length >= 0x80 {
		encoded = append(encoded, byte(length)|0x80)
		length >>= 7
	}
	return append(append(encoded, byte(length)), payload...)
}

func messageToStartTheDebugger(port int) []byte {
	source := append(field(1, fmt.Appendf(nil, scriptThatStartsIt, port)), field(2, []byte(nameOfTheScript))...)
	module := append(field(1, source), field(2, []byte(ModuleOfTheDebugger))...)
	module = append(module, field(3, []byte(ResourceOfTheModule))...)
	return field(1, module)
}

func FreePort(ctx context.Context) (int, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	_, written, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(written)
}

func Attach(ctx context.Context, client *http.Client, enginePort, debuggerPort int) (*Session, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(addressToRunAScript, enginePort), bytes.NewReader(messageToStartTheDebugger(debuggerPort)))
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("asking the game to start its debugger: %w", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the game refused to start its debugger: %s", response.Status)
	}
	return Connect(ctx, net.JoinHostPort("127.0.0.1", strconv.Itoa(debuggerPort)))
}

func Connect(ctx context.Context, address string) (*Session, error) {
	deadline := time.Now().Add(timeToConnect)
	for {
		connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", address)
		if err == nil {
			select {
			case <-time.After(timeToSettle):
			case <-ctx.Done():
				return nil, errors.Join(ctx.Err(), connection.Close())
			}
			session := &Session{connection: connection, replies: make(chan reply, 16), stops: make(chan Stop, 16)}
			go session.listen(bufio.NewReader(connection))
			return session, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return nil, fmt.Errorf("the debugger of the game does not answer on %s: %w", address, err)
		}
		time.Sleep(pauseBetweenAttempts)
	}
}

func (s *Session) Close() {
	_, _ = io.WriteString(s.connection, "EXIT\n")
	_ = s.connection.Close()
}

func (s *Session) listen(answers *bufio.Reader) {
	defer close(s.replies)
	defer close(s.stops)
	for {
		line, err := answers.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if found := paused.FindStringSubmatch(line); found != nil {
			number, _ := strconv.Atoi(found[2])
			s.state.Lock()
			s.isRunning = false
			s.state.Unlock()
			s.stops <- Stop{File: found[1], Line: number}
			continue
		}
		heard := reply{line: line}
		if found := withAPayload.FindStringSubmatch(line); found != nil {
			size, _ := strconv.Atoi(found[1])
			payload := make([]byte, size)
			if _, err := io.ReadFull(answers, payload); err != nil {
				return
			}
			heard.payload = string(payload)
		}
		switch {
		case strings.HasPrefix(line, endOfARun):
			s.state.Lock()
			wasRunning := s.isRunning
			s.isRunning = false
			s.state.Unlock()
			if wasRunning {
				s.stops <- Stop{}
			}
		case !strings.HasPrefix(line, outputOfTheGame):
			s.replies <- heard
		}
	}
}

func (s *Session) isPaused() bool {
	s.state.Lock()
	defer s.state.Unlock()
	return !s.isRunning
}

func (s *Session) ask(command string) (answer reply, err error) {
	s.asking.Lock()
	defer s.asking.Unlock()
	if _, err := io.WriteString(s.connection, command+"\n"); err != nil {
		return reply{}, ErrHasLeft
	}
	select {
	case heard, isThere := <-s.replies:
		switch {
		case !isThere:
			return reply{}, ErrHasLeft
		case strings.HasPrefix(heard.line, "401"):
			return reply{}, errors.New(heard.payload)
		case !strings.HasPrefix(heard.line, "200"):
			return reply{}, fmt.Errorf("the debugger answered %q to %q", heard.line, command)
		}
		heard.line = strings.TrimSpace(strings.TrimPrefix(heard.line, "200 OK"))
		return heard, nil
	case <-time.After(timeToAnswer):
		return reply{}, fmt.Errorf("the debugger did not answer %q", command)
	}
}

func (s *Session) askWhilePaused(command string) (reply, error) {
	if !s.isPaused() {
		return reply{}, ErrIsRunning
	}
	return s.ask(command)
}

func (s *Session) SetBreakpoint(file string, line int) error {
	_, err := s.askWhilePaused(fmt.Sprintf("SETB %s %d", file, line))
	return err
}

func (s *Session) RemoveBreakpoint(file string, line int) error {
	_, err := s.askWhilePaused(fmt.Sprintf("DELB %s %d", file, line))
	return err
}

func (s *Session) Resume(command string) error {
	s.state.Lock()
	wasRunning := s.isRunning
	s.isRunning = true
	s.state.Unlock()
	if wasRunning {
		return ErrIsRunning
	}
	if _, err := s.ask(command); err != nil {
		s.state.Lock()
		s.isRunning = false
		s.state.Unlock()
		return err
	}
	return nil
}

func (s *Session) IsPaused() bool {
	return s.isPaused()
}

func (s *Session) Pause() error {
	if s.isPaused() {
		return nil
	}
	s.asking.Lock()
	defer s.asking.Unlock()
	if _, err := io.WriteString(s.connection, "SUSPEND\n"); err != nil {
		return ErrHasLeft
	}
	return nil
}

func (s *Session) WaitForAStop() (Stop, error) {
	stop, isThere := <-s.stops
	if !isThere {
		return Stop{}, ErrHasLeft
	}
	return stop, nil
}

func (s *Session) Stack() ([]Frame, error) {
	answer, err := s.askWhilePaused(fmt.Sprintf("STACK --{maxlevel=%d}", depthOfValues))
	if err != nil {
		return nil, err
	}
	values, root, err := structureIn(answer.line)
	if err != nil {
		return nil, err
	}
	var frames []Frame
	for _, entry := range values.named(root) {
		info, _ := values.at(entry.value, indexOfTheInfo)
		frame := Frame{}
		if name, isNamed := values.at(info, indexOfTheName); isNamed {
			frame.Name, _ = name.(string)
		}
		if source, isKnown := values.at(info, indexOfTheSource); isKnown {
			frame.File, _ = source.(string)
		}
		if line, isKnown := values.at(info, indexOfTheLine); isKnown {
			number, _ := line.(float64)
			frame.Line = int(number)
		}
		frame.Locals = values.variablesAt(entry.value, indexOfTheLocals)
		frame.Upvalues = values.variablesAt(entry.value, indexOfTheUpvalues)
		frames = append(frames, frame)
	}
	return frames, nil
}

func (s structure) variablesAt(frame any, index float64) []Named {
	held, _ := s.at(frame, index)
	var variables []Named
	for _, variable := range s.named(held) {
		value, _ := s.at(variable.value, 1)
		variables = append(variables, Named{Name: keyAsText(variable.key), Value: s.value(value, depthOfValues)})
	}
	return variables
}

func (s *Session) Evaluate(expression string, frame int) (Value, error) {
	answer, err := s.askWhilePaused(fmt.Sprintf("EXEC return %s --{stack=%d}", strings.ReplaceAll(expression, "\n", " "), frame))
	if err != nil {
		return Value{}, err
	}
	values, root, err := structureIn(answer.payload)
	if err != nil {
		return Value{}, err
	}
	result, _ := values.at(root, 1)
	return values.value(result, depthOfValues), nil
}
