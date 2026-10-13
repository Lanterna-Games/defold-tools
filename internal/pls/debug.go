package pls

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/build"
	"github.com/Lanterna-Games/defold-tools/internal/dap"
	"github.com/Lanterna-Games/defold-tools/internal/mobdebug"
)

const (
	commandThatDebugs    = "dap"
	timeForTheGameToCome = 30 * time.Second
)

type lineWriter struct {
	say     func(text string)
	pending strings.Builder
}

func (l *lineWriter) Write(text []byte) (int, error) {
	l.pending.Write(text)
	for {
		whole := l.pending.String()
		line, rest, hasALine := strings.Cut(whole, "\n")
		if !hasALine {
			return len(text), nil
		}
		l.say(strings.TrimRight(line, "\r"))
		l.pending.Reset()
		l.pending.WriteString(rest)
	}
}

func (w *workshop) launchToDebug(ctx context.Context, asked dap.LaunchArguments, say func(text string)) (*dap.Launched, error) {
	if w.cache == "" {
		return nil, errors.New("this machine has no cache directory to keep the engine and the running game in")
	}
	said := &lineWriter{say: say}
	editor := sync.OnceValues(func() (build.Installation, error) {
		home, _ := os.UserHomeDir()
		return build.FindInstallation(asked.DefoldHome, os.Getenv, home)
	})
	if !asked.IsBuilt {
		installed, err := editor()
		if err != nil {
			return nil, err
		}
		if err := installed.BobToDebug(ctx, asked.Root, w.cache, said); err != nil {
			return nil, err
		}
	}
	engine, err := w.engineFor(asked.Root, asked.Engine, say, editor)
	if err != nil {
		return nil, err
	}
	running, stop := context.WithCancel(ctx)
	served, exited := make(chan int, 1), make(chan struct{})
	go func() {
		defer close(exited)
		if err := build.RunAndTell(running, engine, asked.Root, w.cache, said, func(port int) { served <- port }); err != nil {
			say(programName + ": " + err.Error())
		}
	}()
	var enginePort int
	select {
	case enginePort = <-served:
	case <-exited:
		stop()
		return nil, errors.New("the game stopped before it could be debugged")
	case <-time.After(timeForTheGameToCome):
		stop()
		return nil, errors.New("the game did not say which port it serves")
	}
	debuggerPort, err := mobdebug.FreePort(ctx)
	if err != nil {
		stop()
		return nil, err
	}
	session, err := mobdebug.Attach(running, http.DefaultClient, enginePort, debuggerPort)
	if err != nil {
		stop()
		return nil, err
	}
	return &dap.Launched{Session: session, Stop: stop, Exited: exited}, nil
}

func debugAdapter(stdin io.Reader, stdout io.Writer, shop *workshop, fail func(error) int) int {
	server := &dap.Server{Launch: shop.launchToDebug}
	if err := server.Serve(stdin, stdout); err != nil {
		return fail(err)
	}
	return exitSuccess
}
