package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMessagesAreFramedByTheirLength(t *testing.T) {
	var wire bytes.Buffer
	id := json.RawMessage("7")
	sender := newConnection(strings.NewReader(""), &wire)
	if err := sender.answer(&id, nil); err != nil {
		t.Fatal(err)
	}
	if err := sender.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: "file:///a", Diagnostics: []diagnostic{}}); err != nil {
		t.Fatal(err)
	}

	written := wire.String()
	if !strings.HasPrefix(written, "Content-Length: ") || !strings.Contains(written, "\r\n\r\n{") {
		t.Fatalf("wire = %q, want a length header, a blank line and the body", written)
	}
	// A client waits for "result" to tell an answer from a request of its own, and
	// an empty list of diagnostics is how a file is told it has none left.
	if !strings.Contains(written, `"result":null`) || !strings.Contains(written, `"diagnostics":[]`) {
		t.Errorf("wire = %q, want the null result and the empty list written out", written)
	}

	receiver := newConnection(&wire, &bytes.Buffer{})
	answer, err := receiver.read()
	if err != nil || answer.ID == nil || string(*answer.ID) != "7" {
		t.Errorf("first message = %+v, %v; want the answer to 7", answer, err)
	}
	notification, err := receiver.read()
	if err != nil || notification.Method != "textDocument/publishDiagnostics" || notification.ID != nil {
		t.Errorf("second message = %+v, %v; want the notification", notification, err)
	}
}

func TestAMessageWithoutAUsableLengthIsRefused(t *testing.T) {
	for _, wire := range []string{"\r\n{}", "Content-Length: -4\r\n\r\n{}", "Content-Length: many\r\n\r\n{}", "Content-Length: 99999999999\r\n\r\n{}", "Content-Length: 2\r\n\r\n{"} {
		if received, err := newConnection(strings.NewReader(wire), &bytes.Buffer{}).read(); err == nil {
			t.Errorf("read(%q) = %+v, want it refused", wire, received)
		}
	}
}
