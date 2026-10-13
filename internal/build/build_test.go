package build

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The file the Defold editor keeps next to its launcher, as 1.13.2 writes it.
const configOfAnEditor = `[bootstrap]
resourcespath = 
launcherpath = 

[build]
version = 1.13.2
editor_sha1 = 8620195789abcf164992cc82103a16189d22f675
engine_sha1 = 8620195789abcf164992cc82103a16189d22f675
channel = stable

[launcher]
jdk = ${bootstrap.resourcespath}/packages/jdk-25+36
java = ${launcher.jdk}/bin/java
jar = ${bootstrap.resourcespath}/packages/defold-${build.editor_sha1}.jar
main = com.defold.editor.Main
vmargs = -Dfile.encoding=UTF-8,-Djna.nosys=true,-Ddefold.version=${build.version},--add-opens=java.base/java.lang=ALL-UNNAMED,--enable-native-access=ALL-UNNAMED,-XX:+UseCompactObjectHeaders
`

func editorIn(t *testing.T, directory string, jar map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(directory, "packages"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config"), []byte(configOfAnEditor), 0o600); err != nil {
		t.Fatal(err)
	}
	var written bytes.Buffer
	archive := zip.NewWriter(&written)
	for name, content := range jar {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "packages", "defold-8620195789abcf164992cc82103a16189d22f675.jar"), written.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestAnInstalledEditorSaysWhereItsJavaAndItsJarAre(t *testing.T) {
	home := editorIn(t, t.TempDir(), nil)

	installed, err := InstallationAt(home)

	if err != nil {
		t.Fatal(err)
	}
	java := filepath.Join(home, "packages", "jdk-25+36", "bin", "java")
	if runtime.GOOS == "windows" {
		java += ".exe"
	}
	if installed.Version != "1.13.2" || installed.SHA1 != "8620195789abcf164992cc82103a16189d22f675" || installed.Java != java ||
		installed.Jar != filepath.Join(home, "packages", "defold-8620195789abcf164992cc82103a16189d22f675.jar") {
		t.Errorf("installed = %+v", installed)
	}
	wanted := []string{"-Dfile.encoding=UTF-8", "--add-opens=java.base/java.lang=ALL-UNNAMED", "--enable-native-access=ALL-UNNAMED", "-XX:+UseCompactObjectHeaders"}
	if !slices.Equal(installed.VMArguments, wanted) {
		t.Errorf("arguments for Java = %q, want what Bob needs and nothing of the editor's own: %q", installed.VMArguments, wanted)
	}
	arguments := installed.BobArguments("/games/pong", []string{"--settings", "more.settings"}, "resolve", "build")
	if tail := arguments[len(arguments)-11:]; !slices.Equal(tail, []string{"-cp", installed.Jar, "com.dynamo.bob.Bob", "--root", "/games/pong", "--variant", "debug", "--settings", "more.settings", "resolve", "build"}) {
		t.Errorf("Bob is run with %q", tail)
	}
}

func TestTheEditorIsFoundWhereItSaidItWasInstalled(t *testing.T) {
	old := editorIn(t, filepath.Join(t.TempDir(), "old"), nil)
	current := editorIn(t, filepath.Join(t.TempDir(), "current"), nil)
	asked := editorIn(t, filepath.Join(t.TempDir(), "asked"), nil)
	support := t.TempDir()
	remembered, err := json.Marshal([]knownInstallation{
		{InstallPath: filepath.Join(support, "gone"), LastLaunchedAt: "2026-10-09T10:00:00Z"},
		{InstallPath: old, LastLaunchedAt: "2026-10-01T10:00:00Z"},
		{InstallPath: current, LastLaunchedAt: "2026-10-08T10:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(support, "Defold"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(support, "Defold", "installations.json"), remembered, 0o600); err != nil {
		t.Fatal(err)
	}
	environment := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}

	found, err := FindInstallation("", environment(map[string]string{"LOCALAPPDATA": support}), "")
	if err != nil || found.Home != current {
		t.Errorf("found %q, %v; want the one launched last that is still there, %q", found.Home, err, current)
	}
	found, err = FindInstallation("", environment(map[string]string{"LOCALAPPDATA": support, HomeVariable: asked}), "")
	if err != nil || found.Home != asked {
		t.Errorf("found %q, %v; want the one the environment names", found.Home, err)
	}
	found, err = FindInstallation(old, environment(map[string]string{"LOCALAPPDATA": support, HomeVariable: asked}), "")
	if err != nil || found.Home != old {
		t.Errorf("found %q, %v; want the one that was asked for", found.Home, err)
	}
	if _, err := FindInstallation("", environment(nil), t.TempDir()); !errors.Is(err, ErrNoEditor) {
		t.Errorf("error = %v, want it to say no editor was found", err)
	}
	if _, err := FindInstallation(t.TempDir(), environment(nil), ""); err == nil || !strings.Contains(err.Error(), "does not hold a Defold editor") {
		t.Errorf("error = %v, want a directory without an editor refused", err)
	}
}

func TestTheEngineIsTakenOutOfTheJarOnce(t *testing.T) {
	home := editorIn(t, t.TempDir(), map[string]string{"libexec/x86_64-linux/dmengine": "the engine", "libexec/x86_64-win32/dmengine.exe": "the engine for Windows"})
	installed, err := InstallationAt(home)
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()

	engine, err := installed.engineFor("x86_64-linux", "dmengine", cache)

	content, _ := os.ReadFile(engine)
	if err != nil || string(content) != "the engine" || engine != filepath.Join(cache, installed.SHA1, "x86_64-linux", "dmengine") {
		t.Fatalf("engine = %q holding %q, %v", engine, content, err)
	}
	if info, _ := os.Stat(engine); runtime.GOOS != "windows" && info.Mode()&0o100 == 0 {
		t.Errorf("the engine cannot be run: %v", info.Mode())
	}
	if err := os.Remove(installed.Jar); err != nil {
		t.Fatal(err)
	}
	if again, err := installed.engineFor("x86_64-linux", "dmengine", cache); err != nil || again != engine {
		t.Errorf("the second time gives %q, %v; want the one already taken out", again, err)
	}
	if _, err := installed.engineFor("arm64-linux", "dmengine", cache); err == nil {
		t.Error("an engine the jar does not carry was found")
	}
}

// Bob prints its progress, the trace of every exception and the warnings of the Java it
// runs on. The lines below are from real builds; what is kept is what someone can act on.
func TestWhatBobSaysIsCutDownToWhatMatters(t *testing.T) {
	cases := map[string]struct {
		line string
		want string
		kept bool
	}{
		"an error in a script":    {line: `ERROR: main/ball.script:24: 'com.dynamo.bob.CompileExceptionError: token recognition error at: '?''`, want: `main/ball.script:24:1: token recognition error at: '?'`, kept: true},
		"progress":                {line: ` 50% Reading classes`},
		"the trace of an error":   {line: `2026-10-10 22:20:43 SEVERE  com.dynamo.bob.Project.createTask(Project.java:560) `},
		"what the build is doing": {line: `2026-10-10 22:21:38 INFO    Build tasks `},
		"a warning of Java":       {line: `WARNING: java.lang.System::load has been called by com.dynamo.bob.pipeline.ModelImporterJni in an unnamed module`},
		"a temporary directory":   {line: `Warning: Failed to clean up temp directory 'C:\Users\you\AppData\Local\Temp\12110099749812634901'`},
		"an empty line":           {line: ``},
		"who logs an engine":      {line: `Oct 13, 2026 7:29:51 AM com.defold.extender.client.ExtenderClient log`},
		"an engine being waited":  {line: `INFO: Waiting for job job18008957032036696238 to finish`},
		"an engine that came":     {line: `INFO: Job job18008957032036696238 completed successfully. Writing result to C:\Users\you\AppData\Local\Temp\defold-bob-invoke-1945571663900790136\build.zip`},
		"an engine being asked":   {line: `INFO: Sending async build request to https://build.defold.com/build_async/x86_64-win32/a81dbb0478e8b73931dc2d671563204f6ac0ef82`, want: `INFO: Sending async build request to https://build.defold.com/build_async/x86_64-win32/a81dbb0478e8b73931dc2d671563204f6ac0ef82`, kept: true},
		"anything else":           {line: `Missing main collection`, want: `Missing main collection`, kept: true},
	}
	for name, testCase := range cases {
		if said, isKept := WhatBobSays(testCase.line); isKept != testCase.kept || (isKept && said != testCase.want) {
			t.Errorf("%s: said %q (kept: %v), want %q (kept: %v)", name, said, isKept, testCase.want, testCase.kept)
		}
	}
}

// What Bob 1.14.0 printed when a native extension did not compile, with its progress
// taken out. The compiler's errors come three times over, each followed by the line of
// source it is about, and among them are warnings about Defold's own headers.
const anExtensionThatDoesNotCompile = `Oct 13, 2026 7:30:04 AM com.defold.extender.client.ExtenderClient log
INFO: Sending async build request to https://build.defold.com/build_async/x86_64-win32/a81dbb0478e8b73931dc2d671563204f6ac0ef82
Oct 13, 2026 7:30:05 AM com.defold.extender.client.ExtenderClient log
INFO: Async build request was accepted as job job11061736331134912698 (traceId: 6acdc1dc6735719b9426f43a99d2569d)
Oct 13, 2026 7:30:15 AM com.defold.extender.client.ExtenderClient log
INFO: Job job11061736331134912698 did not complete successfully. Writing log to C:\game\build\x86_64-win32\log.txt

The build failed for the following reasons:

ERROR: myext/ext.manifest:1: '2 warnings and 1 error generated.'

ERROR: myext/ext.manifest:1: '2 warnings and 1 error generated.'

ERROR: myext/src/myext.cpp:8: 'use of undeclared identifier 'nothing_here'
    8 |     return 1 + nothing_here;'

WARNING: myext/ext.manifest:63: 'private field 'd' is not used [-Wunused-private-field]
   63 |     float d;'

ERROR: myext/src/myext.cpp:8: 'use of undeclared identifier 'nothing_here'
    8 |     return 1 + nothing_here;'

WARNING: myext/ext.manifest:63: 'private field 'd' is not used [-Wunused-private-field]
   63 |     float d;'

For the full log, see C:\game\build\x86_64-win32\log.txt (or add -v)
`

func TestAnExtensionThatDoesNotCompileIsSaidOnce(t *testing.T) {
	var listener listenerToBob
	var kept []string
	for line := range strings.SplitSeq(anExtensionThatDoesNotCompile, "\n") {
		if said, isKept := listener.hears(line); isKept {
			kept = append(kept, said)
		}
	}

	want := []string{
		`INFO: Sending async build request to https://build.defold.com/build_async/x86_64-win32/a81dbb0478e8b73931dc2d671563204f6ac0ef82`,
		`INFO: Job job11061736331134912698 did not complete successfully. Writing log to C:\game\build\x86_64-win32\log.txt`,
		`The build failed for the following reasons:`,
		`myext/ext.manifest:1:1: 2 warnings and 1 error generated.`,
		`myext/src/myext.cpp:8:1: use of undeclared identifier 'nothing_here`,
		`    8 |     return 1 + nothing_here;'`,
		`For the full log, see C:\game\build\x86_64-win32\log.txt (or add -v)`,
	}
	if !slices.Equal(kept, want) {
		t.Errorf("kept:\n%s\nwant:\n%s", strings.Join(kept, "\n"), strings.Join(want, "\n"))
	}
}

// Bob leaves the engine it had built for a project with native extensions in a
// directory named after the platform, under the name the engine always has, and
// removes that directory when a build finds no extension. Seen with Bob 1.14.0.
func TestTheEngineOfAProjectIsTheOneItsBuildLeft(t *testing.T) {
	root := t.TempDir()
	if engine, wasBuilt := engineBuiltFor(root, "x86_64-linux", "dmengine"); wasBuilt {
		t.Fatalf("a project that was not built has the engine %q", engine)
	}
	if err := os.MkdirAll(filepath.Join(root, "build", "x86_64-win32", "dmengine.exe"), 0o750); err != nil {
		t.Fatal(err)
	}
	if engine, wasBuilt := engineBuiltFor(root, "x86_64-win32", "dmengine.exe"); wasBuilt {
		t.Errorf("the directory %q was taken for an engine", engine)
	}
	left := filepath.Join(root, "build", "x86_64-linux", "dmengine")
	if err := os.MkdirAll(filepath.Dir(left), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left, []byte("the engine"), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Dir(root))
	engine, wasBuilt := engineBuiltFor(filepath.Base(root), "x86_64-linux", "dmengine")

	if resolved, _ := filepath.EvalSymlinks(left); !wasBuilt || !filepath.IsAbs(engine) || !sameFile(t, engine, resolved) {
		t.Errorf("engine = %q (built: %v), want %q whatever directory the game is started from", engine, wasBuilt, left)
	}
}

func sameFile(t *testing.T, left, right string) bool {
	t.Helper()
	one, errOfOne := os.Stat(left)
	other, errOfOther := os.Stat(right)
	return errOfOne == nil && errOfOther == nil && os.SameFile(one, other)
}

func TestAFileOfTheProjectHasACompiledName(t *testing.T) {
	root := t.TempDir()
	if name, err := CompiledNameOf(root, filepath.Join(root, "main", "ball.script")); err != nil || name != "/main/ball.scriptc" {
		t.Errorf("name = %q, %v", name, err)
	}
	if _, err := CompiledNameOf(root, filepath.Join(filepath.Dir(root), "elsewhere.script")); err == nil {
		t.Error("a file outside the project was given a name in it")
	}
}

func fakeEngine(t *testing.T, port int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in for the engine is a shell script")
	}
	engine := filepath.Join(t.TempDir(), "dmengine")
	script := "#!/bin/sh\necho \"INFO:ENGINE: Engine service started on port " + strconv.Itoa(port) + "\"\necho \"INFO:ENGINE: Loading data from: $1 in $(basename \"$PWD\"), serving on a port that is $DM_SERVICE_PORT\"\nexec sleep 30\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return engine
}

type lockedWriter struct {
	written chan string
}

func (l lockedWriter) Write(text []byte) (int, error) {
	l.written <- string(text)
	return len(text), nil
}

// The engine serves an address on this machine while a game runs, and takes the names of
// compiled files to load again. This was first done by hand against the real engine,
// which answered "/example/tint.scriptc was successfully reloaded".
func TestARunningGameIsRememberedAndToldToReload(t *testing.T) {
	var asked []string
	var body []byte
	game := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		asked = append(asked, request.Method+" "+request.URL.Path)
		body, _ = io.ReadAll(request.Body)
		_, _ = response.Write([]byte("OK"))
	}))
	t.Cleanup(game.Close)
	_, port, _ := net.SplitHostPort(game.Listener.Addr().String())
	listening, _ := strconv.Atoi(port)
	root, cache := filepath.Join(t.TempDir(), "pong"), t.TempDir()
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := Reload(t.Context(), game.Client(), cache, root, []string{"/main/ball.scriptc"}); !errors.Is(err, ErrNothingIsRunning) {
		t.Fatalf("error = %v before the game runs, want it to say nothing is running", err)
	}

	ctx, stop := context.WithCancel(t.Context())
	said := lockedWriter{written: make(chan string, 16)}
	stopped := make(chan error, 1)
	go func() { stopped <- Run(ctx, fakeEngine(t, listening), root, cache, said) }()
	for range 2 {
		select {
		case line := <-said.written:
			if strings.Contains(line, "Loading data") && !strings.Contains(line, filepath.FromSlash("build/default/game.projectc")+" in pong") {
				t.Errorf("the engine was started with %q, want the compiled settings, from the project", line)
			}
			if strings.Contains(line, "Loading data") && !strings.Contains(line, "a port that is dynamic") {
				t.Errorf("the engine was started with %q, want it told to take a port of its own", line)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the engine said nothing")
		}
	}

	if err := Reload(t.Context(), game.Client(), cache, root, []string{"/main/ball.scriptc", "/main/physics.luac"}); err != nil {
		t.Fatal(err)
	}
	wanted := append([]byte{0x0A, 18}, "/main/ball.scriptc"...)
	wanted = append(append(wanted, 0x0A, 18), "/main/physics.luac"...)
	if !slices.Equal(asked, []string{"POST /post/@resource/reload"}) || !bytes.Equal(body, wanted) {
		t.Errorf("the game was asked %q with % x, want one post with % x", asked, body, wanted)
	}

	stop()
	select {
	case err := <-stopped:
		if err != nil {
			t.Errorf("stopping the game: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the game did not stop")
	}
	if err := Reload(t.Context(), game.Client(), cache, root, []string{"/main/ball.scriptc"}); !errors.Is(err, ErrNothingIsRunning) {
		t.Errorf("error = %v after the game stopped, want it to say nothing is running", err)
	}
}
