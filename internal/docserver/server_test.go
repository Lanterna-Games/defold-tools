package docserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

func documentation(version string) (*docs.Reference, error) {
	if version != "1.13.2" && version != "1.9.0" {
		return nil, errors.New("the documentation of Defold " + version + " is not on this machine")
	}
	return &docs.Reference{Version: version, Modules: []docs.Module{
		{Namespace: "base", Name: "Base"},
		{
			Namespace:   "go",
			Name:        "Game object",
			Description: "<p>Functions for <em>game objects</em>.</p>",
			Functions: []docs.Function{{
				Name:        "go.animate",
				Description: "<p>Animates a property of " + version + ".</p>",
				Examples:    "<pre><code>go.animate(url)</code></pre>",
				Parameters:  []docs.Value{{Name: "url", Types: []string{"string", "url"}, Doc: "the <code>url</code>"}, {Name: "delay", Types: []string{"number"}, IsOptional: true}},
				Returns:     []docs.Value{{Name: "id", Types: []string{"hash"}}},
			}},
			Constants:  []docs.Constant{{Name: "go.PLAYBACK_LOOP_FORWARD", Type: "go.PLAYBACK", Doc: "loop forward"}},
			Messages:   []docs.Message{{Name: "set_parent", Fields: []docs.Value{{Name: "parent_id", Types: []string{"hash"}}}}},
			Properties: []docs.Property{{Name: "position", Brief: "the position"}},
		},
	}}, nil
}

func serverForTests() *Server {
	return &Server{Versions: func() []string { return []string{"1.13.2", "1.9.0"} }, Documentation: documentation}
}

func get(t *testing.T, handler http.Handler, method, path string) (status int, body string, location string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, http.NoBody))
	return recorder.Code, recorder.Body.String(), recorder.Header().Get("Location")
}

func TestTheReferenceOpensOnTheNewestVersion(t *testing.T) {
	status, _, location := get(t, serverForTests().Handler(), http.MethodGet, "/")

	if status != http.StatusFound || location != "/1.13.2/" {
		t.Errorf("status = %d, location = %q; want a redirect to the newest version", status, location)
	}
}

func TestAVersionListsItsModules(t *testing.T) {
	status, body, _ := get(t, serverForTests().Handler(), http.MethodGet, "/1.9.0/")

	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	for _, want := range []string{"<h1>Defold 1.9.0</h1>", `href="/1.9.0/go"`, `href="/1.9.0/base"`, `<option value="/1.13.2/">Defold 1.13.2</option>`, `<option value="/1.9.0/" selected>`} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
}

func TestAModuleShowsWhatItDocuments(t *testing.T) {
	status, body, _ := get(t, serverForTests().Handler(), http.MethodGet, "/1.13.2/go")

	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	for _, want := range []string{
		`<h3 id="go.animate"><a href="#go.animate">go.animate(url, [delay])</a></h3>`,
		"<p>Animates a property of 1.13.2.</p>",
		"<p>Functions for <em>game objects</em>.</p>",
		"<td><code>string | url</code></td><td>the <code>url</code></td>",
		`delay <span class="soft">optional</span>`,
		"<pre><code>go.animate(url)</code></pre>",
		`<h3 id="message-set_parent">`,
		`<h3 id="property-position">`,
		`<tr id="go.PLAYBACK_LOOP_FORWARD">`,
		`href="https://defold.com/ref/go-lua"`,
		`<option value="/1.9.0/go">Defold 1.9.0</option>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
}

func TestWhatIsNotThereIsNotFound(t *testing.T) {
	handler := serverForTests().Handler()
	for path, want := range map[string]string{
		"/1.13.2/nothing": `Defold 1.13.2 documents no "nothing"`,
		"/0.1.0/":         "the documentation of Defold 0.1.0 is not on this machine",
		"/0.1.0/go":       "is not on this machine",
	} {
		if status, body, _ := get(t, handler, http.MethodGet, path); status != http.StatusNotFound || !strings.Contains(body, want) {
			t.Errorf("%s: status = %d, body = %q; want not found and %q", path, status, body, want)
		}
	}
	if status, _, _ := get(t, handler, http.MethodPost, "/1.13.2/go"); status != http.StatusMethodNotAllowed {
		t.Errorf("POST: status = %d, want it refused", status)
	}
}

func fetched(t *testing.T, address string) (status int, body string) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err.Error()
	}
	defer response.Body.Close()
	content, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(content)
}

// The port is whatever the system hands out, on this machine only, and it is
// remembered so that links written yesterday still lead somewhere today.
func TestTheReferenceTakesAPortFromTheSystemAndRemembersIt(t *testing.T) {
	portFile := filepath.Join(t.TempDir(), "kept", "reference-port")
	first := serverForTests()
	first.PortFile = portFile
	ctx, stop := context.WithCancel(t.Context())

	address, err := first.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(address, "http://127.0.0.1:") {
		t.Fatalf("address = %q, want it on this machine only", address)
	}
	port, err := strconv.Atoi(strings.TrimPrefix(address, "http://127.0.0.1:"))
	if err != nil || port < 1024 {
		t.Errorf("port = %d (%v), want one the system handed out", port, err)
	}
	if written, _ := os.ReadFile(portFile); strings.TrimSpace(string(written)) != strconv.Itoa(port) {
		t.Errorf("remembered port = %q, want %d", written, port)
	}
	if status, body := fetched(t, address+"/1.13.2/go"); status != http.StatusOK || !strings.Contains(body, "go.animate") {
		t.Errorf("the reference does not answer: %d %q", status, body)
	}

	stop()
	waitUntil(t, "the port is free again", func() bool { status, _ := fetched(t, address+identityPath); return status == 0 })
	again := serverForTests()
	again.PortFile = portFile
	if addressAgain, err := again.Start(t.Context()); err != nil || addressAgain != address {
		t.Errorf("second start = %q, %v; want the remembered %q", addressAgain, err, address)
	}
}

func waitUntil(t *testing.T, what string, isTrue func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if isTrue() {
			return
		}
	}
	t.Fatalf("waited in vain until %s", what)
}

// Two editors are two servers. The second uses the reference the first serves,
// and takes it up when the first one leaves.
func TestASecondServerSharesThePortAndTakesItUpWhenLeftAlone(t *testing.T) {
	portFile := filepath.Join(t.TempDir(), "reference-port")
	first, second := serverForTests(), serverForTests()
	first.PortFile, second.PortFile, second.Pause = portFile, portFile, 20*time.Millisecond
	firstContext, firstLeaves := context.WithCancel(t.Context())

	address, err := first.Start(firstContext)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := second.Start(t.Context())
	if err != nil || shared != address {
		t.Fatalf("second start = %q, %v; want the address of the first, %q", shared, err, address)
	}

	firstLeaves()
	waitUntil(t, "the second serves on the same port", func() bool {
		status, body := fetched(t, address+"/1.9.0/go")
		return status == http.StatusOK && strings.Contains(body, "Animates a property of 1.9.0")
	})
}

func TestAPortSomeoneElseUsesIsLeftAlone(t *testing.T) {
	stranger := httptest.NewServer(http.NotFoundHandler())
	defer stranger.Close()
	portFile := filepath.Join(t.TempDir(), "reference-port")
	if err := os.WriteFile(portFile, []byte(strings.TrimPrefix(stranger.URL, "http://127.0.0.1:")), 0o600); err != nil {
		t.Fatal(err)
	}
	server := serverForTests()
	server.PortFile = portFile

	address, err := server.Start(t.Context())

	if err != nil || address == stranger.URL {
		t.Fatalf("address = %q, %v; want another port than the one in use", address, err)
	}
	if status, _ := fetched(t, address+identityPath); status != http.StatusOK {
		t.Errorf("the reference does not answer on its new port: %d", status)
	}
	if written, _ := os.ReadFile(portFile); "http://127.0.0.1:"+strings.TrimSpace(string(written)) != address {
		t.Errorf("remembered port = %q, want the new one of %s", written, address)
	}
}
