package engine

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	tagOfTheOldVersion    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitOfTheOldVersion = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func archiveOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

type publishedVersions struct {
	server   *httptest.Server
	requests []string
}

func publishingOneOldVersion(t *testing.T) *publishedVersions {
	t.Helper()
	published := &publishedVersions{}
	referenceArchive := archiveOf(t, map[string]string{
		"doc/go_doc.json": `{"info": {"namespace": "go"}, "elements": [
			{"type": "FUNCTION", "name": "go.get", "parameters": [{"name": "url", "types": ["string", "hash", "url"]}, {"name": "property", "types": ["string", "hash"]}]},
			{"type": "MESSAGE", "name": "old_message", "parameters": [{"name": "old_field", "types": ["number"]}]},
			{"type": "PROPERTY", "name": "position"}]}`,
	})
	builtinsArchive := archiveOf(t, map[string]string{
		"builtins/materials/sprite.material": "fragment_constants {\n  name: \"tint\"\n}\n",
		"builtins/input/all.input_binding":   "key_trigger {\n  input: KEY_SPACE\n  action: \"key_space\"\n}\n",
	})
	answers := map[string][]byte{
		"/ref/tags/9.0.0":                                          []byte(`{"object": {"type": "tag", "sha": "` + tagOfTheOldVersion + `"}}`),
		"/tags/" + tagOfTheOldVersion:                              []byte(`{"object": {"type": "commit", "sha": "` + commitOfTheOldVersion + `"}}`),
		"/" + commitOfTheOldVersion + "/engine/share/ref-doc.zip":  referenceArchive,
		"/" + commitOfTheOldVersion + "/engine/share/builtins.zip": builtinsArchive,
	}
	published.server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		published.requests = append(published.requests, request.URL.Path)
		answer, isPublished := answers[request.URL.Path]
		if !isPublished {
			http.NotFound(response, request)
			return
		}
		_, _ = response.Write(answer)
	}))
	t.Cleanup(published.server.Close)
	return published
}

func (p *publishedVersions) versions(cacheDirectory string, announce func(string)) Versions {
	return Versions{
		Publisher:      Publisher{Tags: p.server.URL, Archive: p.server.URL},
		Client:         p.server.Client(),
		CacheDirectory: cacheDirectory,
		Announce:       announce,
	}
}

func TestTheBundledVersionIsServedWithoutAskingAnyone(t *testing.T) {
	published := publishingOneOldVersion(t)
	versions := published.versions(t.TempDir(), nil)

	for _, version := range []string{"", bundled(t).Defold} {
		reference, err := versions.For(version)
		if err != nil || reference.Defold != bundled(t).Defold {
			t.Errorf("For(%q) = %v, %v; want the bundled reference", version, reference, err)
		}
	}
	if len(published.requests) != 0 {
		t.Errorf("requests = %v, want none", published.requests)
	}
}

// Every version of the window is carried by the program, so a project that
// targets one of them needs no network, on the first run or any other.
func TestEveryVersionOfTheWindowIsServedWithoutAskingAnyone(t *testing.T) {
	published := publishingOneOldVersion(t)
	versions := published.versions(t.TempDir(), func(message string) { t.Errorf("announced %q", message) })
	carried := BundledVersions()
	if len(carried) < 30 || carried[0] != OldestSupported {
		t.Fatalf("the program carries %d versions starting at %v, want the window from %v", len(carried), carried[0], OldestSupported)
	}

	for _, version := range carried {
		reference, err := versions.For(version.String())
		if err != nil {
			t.Fatalf("For(%s): %v", version, err)
		}
		if reference.Defold != version.String() || len(reference.SHA1) != 40 {
			t.Errorf("For(%s) is of Defold %q at %q", version, reference.Defold, reference.SHA1)
		}
		if len(reference.Functions) < 500 || len(reference.Messages) < 40 || len(reference.Properties) < 9 || len(reference.URLParameterIndexes("msg.post")) != 1 {
			t.Errorf("the reference of %s is thin: %d functions, %d messages, %d kinds with properties", version, len(reference.Functions), len(reference.Messages), len(reference.Properties))
		}
		if !NamesTheProperty(reference.MaterialProperties, "tint") || len(reference.InputBindings) == 0 {
			t.Errorf("the reference of %s lacks what the builtins hold", version)
		}
	}
	if len(published.requests) != 0 {
		t.Errorf("requests = %v, want none", published.requests)
	}
}

func TestAnotherVersionIsFetchedOnceAndKept(t *testing.T) {
	published := publishingOneOldVersion(t)
	cache := t.TempDir()
	var announced []string
	versions := published.versions(cache, func(message string) { announced = append(announced, message) })

	reference, err := versions.For("9.0.0")
	if err != nil {
		t.Fatal(err)
	}

	if reference.Defold != "9.0.0" || reference.SHA1 != commitOfTheOldVersion {
		t.Errorf("reference is of Defold %q at %q, want 9.0.0 at the commit of its tag", reference.Defold, reference.SHA1)
	}
	if !reference.HasMessage("old_message") || len(reference.Functions["go.get"]) != 2 || !NamesTheProperty(reference.MaterialProperties, "tint") {
		t.Errorf("reference = %+v, want what the archives of that version hold", reference)
	}
	if len(announced) != 1 || !strings.Contains(announced[0], "fetching the reference of Defold 9.0.0") {
		t.Errorf("announced = %v, want one line saying what is fetched", announced)
	}
	if _, err := os.Stat(filepath.Join(cache, "9.0.0.json")); err != nil {
		t.Errorf("the reference was not kept: %v", err)
	}

	requestsOfTheFirstRun := len(published.requests)
	again, err := versions.For("9.0.0")
	if err != nil || again.Defold != "9.0.0" || !again.HasMessage("old_message") {
		t.Errorf("second For = %v, %v; want the same reference", again, err)
	}
	if len(published.requests) != requestsOfTheFirstRun || len(announced) != 1 {
		t.Errorf("the second run asked for %v and announced %v; want it to read what was kept", published.requests[requestsOfTheFirstRun:], announced)
	}
}

func TestAVersionThatCannotBeHadIsRefused(t *testing.T) {
	published := publishingOneOldVersion(t)
	cases := map[string]string{
		"9.9.9":      "version 9.9.9 of Defold was never published",
		"1.9.99":     "version 1.9.99 of Defold was never published",
		"1.5.0":      "the project targets Defold 1.5.0, and the oldest version defold-vet supports is 1.6.0",
		"0.0.1":      "the oldest version defold-vet supports is 1.6.0",
		"latest":     `"latest" is not a Defold version`,
		"../1.0.0":   "is not a Defold version",
		"1.0":        "is not a Defold version",
		"1.0.0-beta": "is not a Defold version",
	}
	for version, want := range cases {
		_, err := published.versions(t.TempDir(), nil).For(version)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("For(%q) error = %v, want it to say %q", version, err, want)
		}
	}
}

func TestAVersionPublishedWithoutItsArchivesIsRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/ref/tags/") {
			_, _ = response.Write([]byte(`{"object": {"type": "commit", "sha": "` + commitOfTheOldVersion + `"}}`))
			return
		}
		http.NotFound(response, request)
	}))
	t.Cleanup(server.Close)
	versions := Versions{Publisher: Publisher{Tags: server.URL, Archive: server.URL}, Client: server.Client()}

	_, err := versions.For("9.1.0")

	if err == nil || !strings.Contains(err.Error(), "version 9.1.0 of Defold was published without the reference archives this build reads") {
		t.Errorf("error = %v, want it to say the archives are missing", err)
	}
}

func TestAReferenceThatCannotBeKeptIsStillUsed(t *testing.T) {
	published := publishingOneOldVersion(t)
	blocked := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	reference, err := published.versions(filepath.Join(blocked, "cache"), nil).For("9.0.0")

	if err != nil || reference.Defold != "9.0.0" {
		t.Errorf("For = %v, %v; want the reference although there is nowhere to keep it", reference, err)
	}
	if got := strconv.Itoa(len(published.requests)); got != "4" {
		t.Errorf("requests = %v, want the tag, the commit and the two archives", published.requests)
	}
}

// A reference with nothing in it would make every check agree with everything.
func TestAReferenceThatDocumentsNothingIsRefused(t *testing.T) {
	empty := archiveOf(t, map[string]string{"doc/go_doc.json": `{"info": {"language": "C++"}, "elements": [{"type": "FUNCTION", "name": "dmGameObject::New"}]}`})

	_, err := FromArchives("0.1.0", commitOfTheOldVersion, empty, archiveOf(t, map[string]string{}))

	if err == nil || !strings.Contains(err.Error(), "the reference of Defold 0.1.0 documents no function") {
		t.Errorf("error = %v, want a refusal", err)
	}
}
