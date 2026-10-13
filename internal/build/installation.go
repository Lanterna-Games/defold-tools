package build

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

const (
	HomeVariable         = "DEFOLD_HOME"
	configOfTheEditor    = "config"
	installationsFile    = "installations.json"
	directoryOfTheEditor = "Defold"
	mainClassOfBob       = "com.dynamo.bob.Bob"
	enginesInTheJar      = "libexec/%s/%s"
	nameOfTheEngine      = "dmengine"
	directoryOfBuilds    = "build"
	suffixOnWindows      = ".exe"
	whereTheEditorIs     = "bootstrap.resourcespath"
	flagOfTheVariant     = "--variant"
	VariantToDebug       = "debug"
	VariantToRelease     = "release"
)

var (
	placeholder = regexp.MustCompile(`\$\{([a-z_]+\.[a-z0-9_]+)\}`)
	platformOf  = map[string]string{"windows/amd64": "x86_64-win32", "linux/amd64": "x86_64-linux", "linux/arm64": "arm64-linux", "darwin/amd64": "x86_64-macos", "darwin/arm64": "arm64-macos"}
	keptForBob  = regexp.MustCompile(`^(--add-opens|--enable-native-access|--sun-misc-unsafe-memory-access|-XX:|-Dfile\.encoding)`)
	ErrNoEditor = errors.New("no Defold editor was found: set " + HomeVariable + " to the directory it is installed in, or pass --defold-home")
)

type Installation struct {
	Home        string
	Version     string
	SHA1        string
	Java        string
	Jar         string
	VMArguments []string
}

func settingsOfTheEditor(home string) (map[string]string, error) {
	file, err := os.Open(filepath.Join(home, configOfTheEditor)) //nolint:gosec // the directory the user says the editor is in
	if err != nil {
		return nil, fmt.Errorf("%s does not hold a Defold editor: %w", home, err)
	}
	defer file.Close()
	settings := map[string]string{}
	section := ""
	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}
		if key, value, isSet := strings.Cut(line, "="); isSet {
			settings[section+"."+strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	settings[whereTheEditorIs] = home
	return settings, lines.Err()
}

func resolved(settings map[string]string, key string) string {
	value := settings[key]
	for range len(settings) {
		replaced := placeholder.ReplaceAllStringFunc(value, func(found string) string {
			return settings[placeholder.FindStringSubmatch(found)[1]]
		})
		if replaced == value {
			break
		}
		value = replaced
	}
	return value
}

func InstallationAt(home string) (Installation, error) {
	settings, err := settingsOfTheEditor(home)
	if err != nil {
		return Installation{}, err
	}
	installed := Installation{
		Home:    home,
		Version: settings["build.version"],
		SHA1:    settings["build.editor_sha1"],
		Java:    filepath.FromSlash(resolved(settings, "launcher.java")),
		Jar:     filepath.FromSlash(resolved(settings, "launcher.jar")),
	}
	for argument := range strings.SplitSeq(resolved(settings, "launcher.vmargs"), ",") {
		if keptForBob.MatchString(argument) {
			installed.VMArguments = append(installed.VMArguments, argument)
		}
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(installed.Java, suffixOnWindows) {
		installed.Java += suffixOnWindows
	}
	if installed.Version == "" || installed.Jar == "" {
		return Installation{}, fmt.Errorf("%s does not say which Defold it holds", filepath.Join(home, configOfTheEditor))
	}
	return installed, nil
}

type knownInstallation struct {
	InstallPath    string `json:"installPath"`
	LastLaunchedAt string `json:"lastLaunchedAt"`
}

func whereEditorsAreRemembered(environment func(string) string, home string) []string {
	var directories []string
	for _, base := range []string{environment("LOCALAPPDATA"), environment("XDG_STATE_HOME"), environment("XDG_CONFIG_HOME")} {
		if base != "" {
			directories = append(directories, filepath.Join(base, directoryOfTheEditor))
		}
	}
	if home != "" {
		directories = append(directories,
			filepath.Join(home, "Library", "Application Support", directoryOfTheEditor),
			filepath.Join(home, ".local", "state", directoryOfTheEditor),
			filepath.Join(home, ".config", directoryOfTheEditor),
			filepath.Join(home, "."+directoryOfTheEditor),
		)
	}
	return directories
}

func FindInstallation(asked string, environment func(string) string, home string) (Installation, error) {
	if asked == "" {
		asked = environment(HomeVariable)
	}
	if asked != "" {
		return InstallationAt(asked)
	}
	var known []knownInstallation
	for _, directory := range whereEditorsAreRemembered(environment, home) {
		content, err := os.ReadFile(filepath.Join(directory, installationsFile)) //nolint:gosec // a file the Defold editor keeps in the user's own directories
		var remembered []knownInstallation
		if err == nil && json.Unmarshal(content, &remembered) == nil {
			known = append(known, remembered...)
		}
	}
	slices.SortFunc(known, func(left, right knownInstallation) int {
		return strings.Compare(right.LastLaunchedAt, left.LastLaunchedAt)
	})
	for _, candidate := range known {
		if installed, err := InstallationAt(candidate.InstallPath); err == nil {
			return installed, nil
		}
	}
	return Installation{}, ErrNoEditor
}

func PlatformOfThisMachine() (platform string, isKnown bool) {
	platform, isKnown = platformOf[runtime.GOOS+"/"+runtime.GOARCH]
	return platform, isKnown
}

func nameOfTheEngineOn(system string) string {
	if system == "windows" {
		return nameOfTheEngine + suffixOnWindows
	}
	return nameOfTheEngine
}

func EngineBuiltFor(root string) (engine string, wasBuilt bool) {
	platform, isKnown := PlatformOfThisMachine()
	if !isKnown {
		return "", false
	}
	return engineBuiltFor(root, platform, nameOfTheEngineOn(runtime.GOOS))
}

func engineBuiltFor(root, platform, name string) (engine string, wasBuilt bool) {
	engine, err := filepath.Abs(filepath.Join(root, directoryOfBuilds, platform, name))
	if err != nil {
		return "", false
	}
	found, err := os.Stat(engine)
	return engine, err == nil && found.Mode().IsRegular()
}

func (i Installation) Engine(cache string) (string, error) {
	platform, isKnown := PlatformOfThisMachine()
	if !isKnown {
		return "", fmt.Errorf("there is no Defold engine for %s on %s", runtime.GOOS, runtime.GOARCH)
	}
	return i.engineFor(platform, nameOfTheEngineOn(runtime.GOOS), cache)
}

func (i Installation) engineFor(platform, name, cache string) (string, error) {
	target := filepath.Join(cache, i.SHA1, platform, name)
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	archive, err := zip.OpenReader(i.Jar)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", i.Jar, err)
	}
	defer archive.Close()
	packed, err := archive.Open(fmt.Sprintf(enginesInTheJar, platform, name))
	if err != nil {
		return "", fmt.Errorf("%s carries no engine for %s: %w", i.Jar, platform, err)
	}
	defer packed.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return "", err
	}
	halfWritten := target + ".part"
	unpacked, err := os.OpenFile(halfWritten, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o750) //nolint:gosec // an engine has to be executable, and the path is under this program's own cache
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(unpacked, packed); err != nil {
		return "", errors.Join(err, unpacked.Close())
	}
	if err := unpacked.Close(); err != nil {
		return "", err
	}
	return target, os.Rename(halfWritten, target)
}

func (i Installation) BobArguments(root string, flags []string, steps ...string) []string {
	arguments := append(slices.Clone(i.VMArguments), "-cp", i.Jar, mainClassOfBob, "--root", root)
	if !slices.Contains(flags, flagOfTheVariant) {
		arguments = append(arguments, flagOfTheVariant, VariantToDebug)
	}
	return append(append(arguments, flags...), steps...)
}
