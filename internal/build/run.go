package build

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // a name for a file, made from the path of a project
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	OutputOfABuild        = "build/default"
	directoryOfBundling   = "build/bundling"
	DirectoryOfBundles    = "dist"
	compiledSettings      = "game.projectc"
	suffixOfACompiledFile = "c"
	reloadAddress         = "http://127.0.0.1:%d/post/@resource/reload"
	fieldOfReloadedPaths  = 0x0A
	timeToReload          = 5 * time.Second
	directoryOfRunning    = "running"
)

var (
	errorOfBob          = regexp.MustCompile(`^ERROR:? ?(.*?):(\d+):? ?'?(.*?)'?$`)
	noiseOfBob          = regexp.MustCompile(`^\s*\d+% |^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d (SEVERE|INFO|WARNING)\s|^WARNING: |^Warning: Failed to clean up temp directory|^\s*$`)
	noiseOfAnEngine     = regexp.MustCompile(` com\.defold\.extender\.[A-Za-z.]+ [A-Za-z]+$|^INFO: (Async build request was accepted|Waiting for job|Checking job result|Job \S+ completed successfully)`)
	restOfWhatWasSaid   = regexp.MustCompile(`^\s+\d+ \| `)
	nameOfAnException   = regexp.MustCompile(`^(?:[a-z]+\.)+[A-Za-z]+(?:Error|Exception): `)
	portOfTheEngine     = regexp.MustCompile(`Engine service started on port (\d+)`)
	ErrNothingIsRunning = errors.New("no game of this project is running: start it with the run command first")
)

func WhatBobSays(line string) (said string, isWorthShowing bool) {
	if found := errorOfBob.FindStringSubmatch(line); found != nil {
		return fmt.Sprintf("%s:%s:1: %s", found[1], found[2], nameOfAnException.ReplaceAllString(found[3], "")), true
	}
	return line, !noiseOfBob.MatchString(line) && !noiseOfAnEngine.MatchString(line)
}

type listenerToBob struct {
	saidBefore  map[string]bool
	isListening bool
}

func (l *listenerToBob) hears(line string) (said string, isWorthShowing bool) {
	if restOfWhatWasSaid.MatchString(line) {
		return line, l.isListening
	}
	said, l.isListening = WhatBobSays(line)
	if l.isListening && errorOfBob.MatchString(line) {
		if l.saidBefore == nil {
			l.saidBefore = map[string]bool{}
		}
		l.isListening = !l.saidBefore[said]
		l.saidBefore[said] = true
	}
	return said, l.isListening
}

const (
	flagOfMoreSettings        = "--settings"
	aPortOfItsOwnForTheEngine = "DM_SERVICE_PORT=dynamic"
	settingsThatBringItIn     = "[bootstrap]\ndebug_init_script = /builtins/scripts/debugger.luac\n"
	fileOfSettingsToDebug     = "debug.settings"
	permissionsOfOurOwnFile   = 0o600
)

func (i Installation) Bundle(ctx context.Context, root, platform, variant, into string, more []string, output io.Writer) error {
	flags := []string{
		flagOfTheVariant, variant, "--platform", platform, "--archive",
		"--output", path.Join(directoryOfBundling, platform), "--bundle-output", into,
	}
	return i.bob(ctx, root, output, append(flags, more...), "resolve", "build", "bundle")
}

func (i Installation) BobToDebug(ctx context.Context, root, cache string, output io.Writer) error {
	settings := filepath.Join(cache, fileOfSettingsToDebug)
	if err := os.MkdirAll(cache, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(settings, []byte(settingsThatBringItIn), permissionsOfOurOwnFile); err != nil {
		return err
	}
	return i.bob(ctx, root, output, []string{flagOfMoreSettings, settings}, "build")
}

func (i Installation) Bob(ctx context.Context, root string, output io.Writer, steps ...string) error {
	return i.bob(ctx, root, output, nil, steps...)
}

func (i Installation) bob(ctx context.Context, root string, output io.Writer, flags []string, steps ...string) error {
	command := exec.CommandContext(ctx, i.Java, i.BobArguments(root, flags, steps...)...) //nolint:gosec // the Java and the jar of the editor the user installed
	command.Dir = root
	said, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		return fmt.Errorf("starting the build with %s: %w", i.Java, err)
	}
	lines := bufio.NewScanner(said)
	lines.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var listener listenerToBob
	for lines.Scan() {
		if shown, isWorthShowing := listener.hears(lines.Text()); isWorthShowing {
			fmt.Fprintln(output, shown)
		}
	}
	if err := command.Wait(); err != nil {
		return errors.New("the build failed")
	}
	return nil
}

type running struct {
	Port int `json:"port"`
}

func whereItIsRemembered(cache, root string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	digest := sha1.Sum([]byte(filepath.ToSlash(absolute))) //nolint:gosec // as above
	return filepath.Join(cache, directoryOfRunning, hex.EncodeToString(digest[:])+".json")
}

func Run(ctx context.Context, engine, root, cache string, output io.Writer) error {
	return RunAndTell(ctx, engine, root, cache, output, func(int) {})
}

func RunAndTell(ctx context.Context, engine, root, cache string, output io.Writer, servesOn func(port int)) error {
	command := exec.CommandContext(ctx, engine, filepath.Join(filepath.FromSlash(OutputOfABuild), compiledSettings)) //nolint:gosec // the engine of the editor the user installed, the one their build made, or the one named on the command line
	command.Dir = root
	command.Env = append(os.Environ(), aPortOfItsOwnForTheEngine)
	said, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", engine, err)
	}
	remembered := whereItIsRemembered(cache, root)
	defer os.Remove(remembered)
	lines := bufio.NewScanner(said)
	lines.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lines.Scan() {
		line := lines.Text()
		fmt.Fprintln(output, line)
		if found := portOfTheEngine.FindStringSubmatch(line); found != nil {
			port, _ := strconv.Atoi(found[1])
			remember(remembered, running{Port: port})
			servesOn(port)
		}
	}
	if err := command.Wait(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("the game stopped: %w", err)
	}
	return nil
}

func remember(file string, game running) {
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return
	}
	if encoded, err := json.Marshal(game); err == nil {
		_ = os.WriteFile(file, encoded, 0o600)
	}
}

func CompiledNameOf(root, file string) (resourcePath string, err error) {
	absoluteRoot, errOfRoot := filepath.Abs(root)
	absoluteFile, errOfFile := filepath.Abs(file)
	if errOfRoot != nil || errOfFile != nil {
		return "", fmt.Errorf("%s cannot be placed in %s", file, root)
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteFile)
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", fmt.Errorf("%s is not a file of the project in %s", file, root)
	}
	return "/" + filepath.ToSlash(relative) + suffixOfACompiledFile, nil
}

func messageToReload(resourcePaths []string) []byte {
	var message bytes.Buffer
	for _, resourcePath := range resourcePaths {
		message.WriteByte(fieldOfReloadedPaths)
		length := len(resourcePath)
		for length >= 0x80 {
			message.WriteByte(byte(length) | 0x80)
			length >>= 7
		}
		message.WriteByte(byte(length))
		message.WriteString(resourcePath)
	}
	return message.Bytes()
}

func Reload(ctx context.Context, client *http.Client, cache, root string, resourcePaths []string) error {
	content, err := os.ReadFile(whereItIsRemembered(cache, root))
	var game running
	if err != nil || json.Unmarshal(content, &game) != nil || game.Port == 0 {
		return ErrNothingIsRunning
	}
	ctx, cancel := context.WithTimeout(ctx, timeToReload)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(reloadAddress, game.Port), bytes.NewReader(messageToReload(resourcePaths)))
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%w (the game on port %d does not answer)", ErrNothingIsRunning, game.Port)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("the game refused to reload: %s", response.Status)
	}
	return nil
}
