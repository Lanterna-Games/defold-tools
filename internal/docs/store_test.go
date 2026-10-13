package docs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shaOfAnOlderVersion = "cccccccccccccccccccccccccccccccccccccccc"

func archiveOfOneModule(t *testing.T) []byte {
	t.Helper()
	return archiveOf(t, map[string]string{
		"doc/go_doc.json": `{"info": {"namespace": "go", "name": "Game object"}, "elements": [{"type": "FUNCTION", "name": "go.get"}]}`,
	})
}

func carrying(version string) func() (string, func() (*Reference, error), bool) {
	return func() (string, func() (*Reference, error), bool) {
		return version, func() (*Reference, error) { return &Reference{Version: version}, nil }, true
	}
}

func TestTheCarriedDocumentationNeedsNoArchive(t *testing.T) {
	store := Store{Carried: carrying("1.13.2"), Fetch: func(string) ([]byte, error) { return nil, errors.New("no network in tests") }}

	reference, err := store.For("1.13.2", "any")

	if err != nil || reference.Version != "1.13.2" || !store.IsAtHand("1.13.2", "any") {
		t.Errorf("For = %+v, %v; want the documentation the program carries", reference, err)
	}
}

func TestAnotherVersionIsFetchedOnceAndKept(t *testing.T) {
	archives := filepath.Join(t.TempDir(), "archives")
	timesFetched := 0
	var announced []string
	store := Store{
		Archives: archives,
		Carried:  carrying("1.13.2"),
		Fetch: func(sha1 string) ([]byte, error) {
			timesFetched++
			if sha1 != shaOfAnOlderVersion {
				t.Errorf("fetched %q, want the archive of the version asked for", sha1)
			}
			return archiveOfOneModule(t), nil
		},
		Announce: func(message string) { announced = append(announced, message) },
	}
	if store.IsAtHand("1.9.0", shaOfAnOlderVersion) {
		t.Fatal("the documentation is said to be at hand before it was fetched")
	}

	for range 2 {
		reference, err := store.For("1.9.0", shaOfAnOlderVersion)
		if err != nil || reference.Version != "1.9.0" || reference.Modules[0].Namespace != "go" {
			t.Fatalf("For = %+v, %v", reference, err)
		}
	}

	if timesFetched != 1 || len(announced) != 1 || !strings.Contains(announced[0], "fetching the documentation of Defold 1.9.0") {
		t.Errorf("fetched %d times and announced %v; want once each", timesFetched, announced)
	}
	if _, err := os.Stat(filepath.Join(archives, shaOfAnOlderVersion+"-ref-doc.zip")); err != nil || !store.IsAtHand("1.9.0", shaOfAnOlderVersion) {
		t.Errorf("the archive was not kept: %v", err)
	}
}

func TestDocumentationThatCannotBeHadIsRefused(t *testing.T) {
	offline := Store{Archives: t.TempDir(), Carried: carrying("1.13.2")}
	if _, err := offline.For("1.9.0", shaOfAnOlderVersion); err == nil || !strings.Contains(err.Error(), "the documentation of Defold 1.9.0 is not on this machine") {
		t.Errorf("error = %v, want it to say the documentation is not there", err)
	}

	failing := Store{Fetch: func(string) ([]byte, error) { return nil, errors.New("no route to host") }}
	if _, err := failing.For("1.9.0", shaOfAnOlderVersion); err == nil || !strings.Contains(err.Error(), "no route to host") {
		t.Errorf("error = %v, want the reason the fetch failed", err)
	}
}

func TestDocumentationSurvivesPacking(t *testing.T) {
	reference, err := FromArchive("1.9.0", archiveOfOneModule(t))
	if err != nil {
		t.Fatal(err)
	}

	packed, err := Pack(reference)
	if err != nil {
		t.Fatal(err)
	}
	unpacked, err := Unpack(packed)

	if err != nil || unpacked.Version != "1.9.0" || unpacked.Modules[0].Functions[0].Name != "go.get" {
		t.Errorf("unpacked = %+v, %v", unpacked, err)
	}
	if _, err := Unpack([]byte("not packed")); err == nil {
		t.Error("something that was never packed was unpacked")
	}
}
